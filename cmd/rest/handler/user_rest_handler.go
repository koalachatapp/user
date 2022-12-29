package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/koalachatapp/user/internal/core/entity"
	"github.com/koalachatapp/user/internal/core/port"
)

type RestHandler struct {
	service port.UserService
}

func NewRestHandler(service port.UserService) *RestHandler {
	return &RestHandler{
		service: service,
	}
}

func (h *RestHandler) Post(ctx *fiber.Ctx) error {
	user := &entity.UserEntity{}
	ctx.BodyParser(user)
	var error_msg []string
	if len(user.Username) == 0 {
		error_msg = append(error_msg, "Username is required")
	}
	if len(user.Password) == 0 {
		error_msg = append(error_msg, "Password is required")
	}
	if len(user.Email) == 0 {
		error_msg = append(error_msg, "Email is required")
	}
	if len(user.Name) == 0 {
		error_msg = append(error_msg, "User is required")
	}
	if len(error_msg) > 0 {
		return ctx.JSON(map[string]string{
			"status":  "error",
			"message": strings.Join(error_msg, ";"),
		})
	}
	err := h.service.Register(*user)
	if err != nil {
		return ctx.JSON(map[string]string{"status": "error", "message": err.Error()})
	}
	return ctx.JSON(map[string]string{
		"status": "success",
	})
}

func (h *RestHandler) Delete(ctx *fiber.Ctx) error {
	return nil
}
