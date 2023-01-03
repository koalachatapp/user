package repository

import (
	"log"
	"os"
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
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "koala"
		}
		pass := os.Getenv("DB_PASS")
		if pass == "" {
			pass = "ko4la"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "koala"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		dsn := "host=" + host + " user=" + user + " password=" + pass + " dbname=" + dbname + " port=" + port + " sslmode=disable TimeZone=Asia/Jakarta"
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
	var users entity.UserEntity
	tx := u.db.Model(&users).Where("uuid=?", uuid).Updates(user)
	return tx.Error
}

func (u *userRepository) Patch(uuid string, user entity.UserEntity) error {
	var users entity.UserEntity
	if user.Username != "" {
		u.db.Model(users).Where("uuid=?", uuid).UpdateColumn("username", user.Username)
	}
	if err := func(param ...[2]string) error {
		for _, param := range param {
			if param[0] != "" && param[1] != "" {
				if tx := u.db.Model(users).Where("uuid=? and "+param[0]+"=?", uuid, param[1]).Update(param[0], param[1]); tx.Error != nil {
					return tx.Error
				}
			}
		}
		return nil
	}(
		[2]string{"email", user.Email},
		[2]string{"name", user.Name},
		[2]string{"password", user.Password},
		[2]string{"username", user.Username},
	); err != nil {
		return err
	}
	return nil
}

func (u *userRepository) Delete(uuid string) (bool, error) {
	var user entity.UserEntity
	tx := u.db.Where("uuid=?", uuid).Delete(&user)
	if tx.RowsAffected == 1 {
		return true, nil
	}
	return false, tx.Error
}

func (u *userRepository) IsExistUuid(uuid string) (bool, error) {
	var users []entity.UserEntity
	tx := u.db.Where("uuid=?", uuid).Find(&users)
	if len(users) == 1 {
		return true, nil
	}
	return false, tx.Error
}

func (u *userRepository) IsExist(username string, email string) (bool, error) {

	var users []entity.UserEntity
	tx := u.db.Where("username=? OR email=?", username, email).Find(&users)
	if tx.Error != nil {
		return false, tx.Error
	}
	if len(users) == 1 {
		return true, nil
	}
	return false, nil
}
