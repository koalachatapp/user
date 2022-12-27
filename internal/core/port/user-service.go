package port

import "github.com/koalachatapp/user/internal/core/entity"

type UserService interface {
	Register(user entity.UserEntity) error
}
