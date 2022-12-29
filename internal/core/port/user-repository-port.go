package port

import "github.com/koalachatapp/user/internal/core/entity"

type UserRepository interface {
	Save(user entity.UserEntity) error
	Delete(uuid string) error
	IsExist(username string, email string) (bool, error)
}
