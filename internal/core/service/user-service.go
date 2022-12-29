package service

import (
	"errors"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
)

type userService struct {
	repository port.UserRepository
	msg        chan func() error
	wg         sync.WaitGroup
}

var storage []func() error

func NewUserService(repository port.UserRepository) port.UserService {
	userservice := &userService{
		repository: repository,
		msg:        make(chan func() error, 200),
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
			}(u.msg, &u.wg)
		}
	}(userservice)
	// retry mechanism
	go func(u *userService) {
		for {
			if len(storage) > 0 {
				log.Printf("%d pending task\n", len(storage))
				for i := 0; i < len(storage); i++ {
					u.wg.Add(1)
					u.msg <- storage[i]
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
	u, _ := ioutil.ReadFile("/proc/sys/kernel/random/uuid")
	user.Uuid = string(u)
	exist, err := s.repository.IsExist(user.Username, user.Email)
	if err != nil {
		log.Println(err)
		return errors.New("faiied connect to DB")
	}
	if exist {
		return errors.New("user already registered")
	}
	s.wg.Add(1)
	s.msg <- func() error {
		return s.repository.Save(user)
	}
	return nil
}
