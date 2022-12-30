package service

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/jellydator/ttlcache/v3"
	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
)

type userService struct {
	repository port.UserRepository
	cache      *ttlcache.Cache[uint8, []string]
	worker     *port.Worker
	prod       sarama.SyncProducer
}

var storage []func() error

func NewUserService(repository port.UserRepository, worker *port.Worker) port.UserService {
	userservice := &userService{
		repository: repository,
		worker:     worker,
		cache:      ttlcache.New[uint8, []string](),
	}
	go userservice.cache.Start()
	userservice.cache.Set(0, []string{}, ttlcache.NoTTL)
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			userservice.cache.DeleteAll()
			userservice.cache.Set(0, []string{}, ttlcache.NoTTL)
		}
	}()
	// worker
	go func(u *userService) {
		for i := 0; i < 10; i++ {
			go func(msg <-chan func() error, wg *sync.WaitGroup) {
				for m := range msg {
					err := m()
					if err != nil {
						log.Println(err)
						storage = append(storage, m)
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
					u.worker.Worker <- storage[i]
				}
				storage = nil
			}
			time.Sleep(3 * time.Second)
		}
	}(userservice)
	saramaconfig := sarama.NewConfig()
	saramaconfig.Producer.Return.Successes = true
	saramaconfig.Producer.Retry.Max = 0
	saramaddr := os.Getenv("KAFKA_URL")
	if saramaddr == "" {
		saramaddr = "kafka:9092"
	}
	prod, err := sarama.NewSyncProducer([]string{saramaddr}, saramaconfig)
	if err != nil {
		log.Println(err)
	}
	userservice.prod = prod
	return userservice
}

func (s *userService) Register(user entity.UserEntity) error {
	if err := validateNotEmpty(
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

	exist, err := s.repository.IsExist(user.Username, user.Email)
	if err != nil {
		log.Println(err)
		return errors.New("failed connect to DB")
	}
	if exist {
		return errors.New("user already registered")
	}
	u, _ := ioutil.ReadFile("/proc/sys/kernel/random/uuid")

	s.worker.Wg.Add(2)
	s.worker.Worker <- func() error {
		user.Uuid = strings.TrimSpace(string(u))
		s.cache.Set(0, append(s.cache.Get(0).Value(), user.Uuid), ttlcache.NoTTL)
		sha := sha512.New()
		var reverseUuid2byte []byte
		for i := len([]rune(user.Uuid)) - 1; i == 0; i-- {
			reverseUuid2byte = append(reverseUuid2byte, user.Uuid[i])
		}
		sha.Write([]byte(user.Uuid + "." + base64.StdEncoding.EncodeToString(reverseUuid2byte) + "." + user.Password))
		user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
		return s.repository.Save(user)
	}
	s.worker.Worker <- func() error {
		part, off, err := s.prod.SendMessage(&sarama.ProducerMessage{
			Topic: "Topic1",
			Value: sarama.StringEncoder("hello"),
		})
		log.Printf("Sending message to Topic1 on partition %d at offset %d\n", part, off)
		return err
	}
	return nil
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
	success, err := s.repository.Delete(uuid)
	if !success {
		if err != nil {
			log.Println(err)
			return errors.New("failed to delete user")
		}
		return errors.New("uuid not found")
	}
	cur := s.cache.Get(0).Value()
	for k, v := range cur {
		if v == uuid {
			cur[k] = cur[len(cur)-1]
			cur = cur[:len(cur)-1]
			break
		}
	}
	s.cache.Set(0, cur, ttlcache.NoTTL)
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
		s.cache.Set(0, append(s.cache.Get(0).Value(), uuid), ttlcache.NoTTL)
	}
	if user.Password != "" {
		sha := sha512.New()
		sha.Write([]byte(uuid + "." + base64.StdEncoding.EncodeToString([]byte(uuid)) + "." + user.Password))
		user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- func() error {
		return s.repository.Update(uuid, user)
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
		s.cache.Set(0, append(s.cache.Get(0).Value(), uuid), ttlcache.NoTTL)
	}

	if user.Password != "" {
		sha := sha512.New()
		sha.Write([]byte(uuid + "." + base64.StdEncoding.EncodeToString([]byte(uuid)) + "." + user.Password))
		user.Password = base64.RawURLEncoding.EncodeToString(sha.Sum(nil))
	}
	s.worker.Wg.Add(1)
	s.worker.Worker <- func() error {
		return s.repository.Patch(uuid, user)
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
	found := false
	for _, v := range s.cache.Get(0).Value() {
		if v == uuid {
			found = true
			return true, nil
		}
	}
	if !found {
		exist, err := s.repository.IsExistUuid(uuid)
		if !exist {
			if err != nil {
				log.Println(err)
				return false, errors.New("failed delete user")
			}
			return false, errors.New("uuid not found")
		}
	}
	return false, nil
}
