package port

import "github.com/koalachatapp/user/internal/core/entity"

type UserService interface {
	Register(user entity.UserEntity) (string, error)
	Update(uuid string, user entity.UserEntity) error
	Patch(uuid string, user entity.UserEntity) error
	Delete(uuid string) error
}
