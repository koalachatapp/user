package service

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bytedance/sonic"
	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
)

type userService struct {
	repository port.UserRepository
	worker     *port.Worker
	prod       sarama.AsyncProducer
	redis      *redis.Client
}

var storage []func() error

// NewUserService creates a new user service
func NewUserService(repository port.UserRepository, worker *port.Worker) port.UserService {
	userservice := &userService{
		repository: repository,
		worker:     worker,
	}
	userservice.redis = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// worker
	go func(u *userService) {
		for i := 0; i < 10; i++ {
			go func(msg <-chan map[uint8]interface{}, wg *sync.WaitGroup) {
				for msg := range msg {
					for k, v := range msg {
						switch k {
						case 0:
							err := v.(func() error)()
							if err != nil {
								log.Println(err)
								storage = append(storage, v.(func() error))
							}
						case 1:
							log.Println("Sending to Kafka : ", fmt.Sprintf("%s", v))
							(u.worker.Prod).SendMessage(&sarama.ProducerMessage{
								Topic: "UsersearchTopic",
								Value: sarama.StringEncoder(fmt.Sprintf("%s", v)),
							})
						}
					}
					wg.Done()
				}
			}(u.worker.Worker, &u.worker.Wg)
		}
	}(userservice)
	// retry mechanism
	go func(u *userService) {
		for {
			if len(storage) > 0 {
				log.Printf("rerunning %d pending task..\n", len(storage))
				for i := 0; i < len(storage); i++ {
					u.worker.Wg.Add(1)
					u.worker.Worker <- map[uint8]interface{}{
						0: storage[i],
					}
				}
				storage = nil
			}
			time.Sleep(3 * time.Second)
		}
	}(userservice)

	return userservice
}

func (s *userService) Register(user entity.UserEntity) (string, error) {
	if err := validateNotEmpty(
		[2]string{"username", user.Username},
		[2]string{"password", user.Password},
		[2]string{"email", user.Email},
		[2]string{"name", user.Name},
	); err != nil {
		return "", err
	}
	isvalid := make(chan bool, 5)
	go func() {
		isvalid <- validateEmail(user.Email)
	}()
	if !<-isvalid {
		return "", errors.New("invalid email address")
	}
	email := s.redis.Get(context.TODO(), user.Email)
	username := s.redis.Get(context.TODO(), user.Username)
	if email.Err() != nil || username.Err() != nil {
		exist, err := s.repository.IsExist(user.Username, user.Email)
		if err != nil {
			log.Println(err)
			return "", errors.New("failed connect to DB")
		}
		if exist {
			return "", errors.New("user already registered")
		}
		u := uuid.New().String()
		user.Uuid = strings.TrimSpace(string(u))
		s.redis.SetNX(context.Background(), user.Email, true, time.Minute)
		s.redis.SetNX(context.Background(), user.Username, true, time.Minute)
	}
	if email.Val() != "" || username.Val() != "" {
		return "", errors.New("user already registered")
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		0: func() error {
			sha := sha512.New()
			var reverseUUID2byte []byte
			for i := len([]rune(user.Uuid)) - 1; i == 0; i-- {
				reverseUUID2byte = append(reverseUUID2byte, user.Uuid[i])
			}
			sha.Write([]byte(user.Uuid + "." + base64.StdEncoding.EncodeToString(reverseUUID2byte) + "." + user.Password))
			user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
			b, err := sonic.Marshal(&user)
			if err != nil {
				return err
			}
			success, err := s.redis.SetNX(context.Background(), user.Uuid, b, 1*time.Minute).Result()
			if err != nil {
				return err
			}
			if success {
				log.Println("chaching on redis")
			}
			data := entity.UserEventEntity{
				Method: "register",
				Data:   user,
			}
			json, err := sonic.Marshal(&data)
			if err != nil {
				return err
			}
			s.worker.Wg.Add(1)
			s.worker.Worker <- map[uint8]interface{}{
				1: json,
			}
			return s.repository.Save(user)
		},
	}

	return user.Uuid, nil
}

func (s *userService) Delete(uuid string) error {
	if err := validateNotEmpty(
		[2]string{"uuid", uuid},
	); err != nil {
		return err
	}
	if _, err := s.checkUuid(uuid); err != nil {
		return err
	}
	res, err := s.redis.Del(context.TODO(), uuid).Result()
	if err != nil {
		return err
	}
	log.Println(res)
	success, err := s.repository.Delete(uuid)
	if !success {
		if err != nil {
			log.Println(err)
			return errors.New("failed to delete user")
		}
		return errors.New("uuid not found")
	}
	var user entity.UserEntity = entity.UserEntity{
		Uuid: uuid,
	}
	data := entity.UserEventEntity{
		Method: "delete",
		Data:   user,
	}
	json, _ := sonic.Marshal(&data)
	if err != nil {
		return err
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		1: json,
	}
	return nil
}

func (s *userService) Update(uuid string, user entity.UserEntity) error {
	if err := validateNotEmpty(
		[2]string{"uuid", uuid},
		[2]string{"username", user.Username},
		[2]string{"password", user.Password},
		[2]string{"email", user.Email},
		[2]string{"name", user.Name},
	); err != nil {
		return err
	}
	if !validateEmail(user.Email) {
		return errors.New("invalid email address")
	}
	cached, err := s.checkUuid(uuid)
	if err != nil {
		return err
	}
	if !cached {
		b, err := sonic.Marshal(&user)
		if err != nil {
			return err
		} else {
			s.redis.SetNX(context.Background(), uuid, b, 2*time.Minute)
			log.Printf("chacing uuid on redis [%s]\n", uuid)
		}
	}
	if user.Password != "" {
		sha := sha512.New()
		sha.Write([]byte(uuid + "." + base64.StdEncoding.EncodeToString([]byte(uuid)) + "." + user.Password))
		user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		0: func() error {
			return s.repository.Update(uuid, user)
		},
	}
	user.Uuid = uuid
	data := entity.UserEventEntity{
		Method: "update",
		Data:   user,
	}
	json, _ := sonic.Marshal(&data)
	if err != nil {
		return err
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		1: json,
	}
	return nil
}

func (s *userService) Patch(uuid string, user entity.UserEntity) error {
	if err := validateNotEmpty([2]string{"uuid", uuid}); err != nil {
		return err
	}
	if user.Email == "" && user.Name == "" && user.Password == "" && user.Username == "" {
		return errors.New("at least one data must be changed")
	}
	if user.Email != "" {
		if !validateEmail(user.Email) {
			return errors.New("invalid email address")
		}
	}
	cached, err := s.checkUuid(uuid)
	if err != nil {
		return err
	}
	if !cached {
		b, err := sonic.Marshal(&user)
		if err != nil {
			return err
		} else {
			s.redis.SetNX(context.Background(), uuid, b, 2*time.Minute)
			log.Printf("chacing uuid on redis [%s]\n", uuid)
		}
	}

	if user.Password != "" {
		sha := sha512.New()
		sha.Write([]byte(uuid + "." + base64.StdEncoding.EncodeToString([]byte(uuid)) + "." + user.Password))
		user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		0: func() error {
			return s.repository.Patch(uuid, user)
		},
	}
	user.Uuid = uuid
	data := entity.UserEventEntity{
		Method: "patch",
		Data:   user,
	}
	json, _ := sonic.Marshal(&data)
	if err != nil {
		return err
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- map[uint8]interface{}{
		1: json,
	}
	return nil
}

// helper
func validateNotEmpty(param ...[2]string) error {
	var error_msg []string
	for _, v := range param {
		if v[1] == "" {
			error_msg = append(error_msg, v[0]+" cannot be empty")
		}
	}
	if len(error_msg) > 0 {
		return errors.New(strings.Join(error_msg, ";"))
	}
	return nil
}

func validateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return emailRegex.MatchString(email)
}

func (s *userService) checkUuid(uuid string) (bool, error) {
	data, err := s.redis.Get(context.TODO(), uuid).Result()
	if err == nil && data != "" {
		log.Println("uuid found on redis")
		return true, nil
	}

	exist, err := s.repository.IsExistUuid(uuid)
	if !exist {
		if err != nil {
			log.Println(err)
			return false, errors.New("failed delete user")
		}
		return false, errors.New("uuid not found")
	}

	return false, nil
}
