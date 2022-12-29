package port

import "github.com/koalachatapp/user/internal/core/entity"

type UserRepository interface {
	Save(user entity.UserEntity) error
	Delete(uuid string) (bool, error)
	IsExist(username string, email string) (bool, error)
	IsExistUuid(uuid string) (bool, error)
	Update(uuid string, user entity.UserEntity) error
	Patch(uuid string, user entity.UserEntity) error
}
