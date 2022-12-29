package repository

import (
	"log"
	"sync"

	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type userRepository struct {
	once sync.Once
	db   *gorm.DB
}

var repo *userRepository = &userRepository{
	once: sync.Once{},
}

func NewUserRepository() port.UserRepository {
	repo.once.Do(func() {
		dsn := "host=localhost user=koala password=ko4la dbname=koala port=5432 sslmode=disable TimeZone=Asia/Jakarta"
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
			// Logger:  logger.Default.LogMode(logger.Error),
			SkipDefaultTransaction: true,
		})
		db.AutoMigrate(&entity.UserEntity{})
		if err != nil {
			log.SetPrefix("[Warning] ")
			log.Println(err)
		}
		repo.db = db
	})
	return repo
}

func (u *userRepository) Save(user entity.UserEntity) error {
	res := u.db.Create(&user)
	return res.Error
}

func (u *userRepository) Update(uuid string, user entity.UserEntity) error {
	return nil
}

func (u *userRepository) Delete(uuid string) error {
	return nil
}

func (u *userRepository) IsExist(username string, email string) (bool, error) {

	var users []entity.UserEntity
	tx := u.db.Where("username=? OR email=?", username, email).Find(&users)
	if tx.Error != nil {
		return false, tx.Error
	}
	if len(users) > 0 {
		return true, nil
	}
	return false, nil
}
