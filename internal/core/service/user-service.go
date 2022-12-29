package service

import (
	"errors"
	"io/ioutil"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
)

type userService struct {
	repository port.UserRepository
	worker     chan func() error
	wg         sync.WaitGroup
}

var storage []func() error

func NewUserService(repository port.UserRepository) port.UserService {
	userservice := &userService{
		repository: repository,
		worker:     make(chan func() error, 200),
	}
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
			}(u.worker, &u.wg)
		}
	}(userservice)
	// retry mechanism
	go func(u *userService) {
		for {
			if len(storage) > 0 {
				log.Printf("rerunning %d pending task..\n", len(storage))
				for i := 0; i < len(storage); i++ {
					u.wg.Add(1)
					u.worker <- storage[i]
				}
				storage = nil
			}
			time.Sleep(3 * time.Second)
		}
	}(userservice)
	userservice.wg.Wait()
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
	u, _ := ioutil.ReadFile("/proc/sys/kernel/random/uuid")
	user.Uuid = strings.TrimSpace(string(u))

	exist, err := s.repository.IsExist(user.Username, user.Email)
	if err != nil {
		log.Println(err)
		return errors.New("failed connect to DB")
	}
	if exist {
		return errors.New("user already registered")
	}
	s.wg.Add(1)
	s.worker <- func() error {
		return s.repository.Save(user)
	}
	return nil
}

func (s *userService) Delete(uuid string) error {
	if err := validateNotEmpty(
		[2]string{"uuid", uuid},
	); err != nil {
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
	exist, err := s.repository.IsExistUuid(uuid)
	if !exist {
		if err != nil {
			log.Println(err)
			return errors.New("failed update user")
		}
		return errors.New("uuid not found")
	}
	s.wg.Add(1)
	s.worker <- func() error {
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
	exist, err := s.repository.IsExistUuid(uuid)
	if !exist {
		if err != nil {
			log.Println(err)
			return errors.New("failed update user")
		}
		return errors.New("uuid not found")
	}
	s.wg.Add(1)
	s.worker <- func() error {
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
