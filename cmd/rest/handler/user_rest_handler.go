package handler

import (
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

	err := h.service.Register(*user)
	if err != nil {
		if err.Error() == "user already registered" {
			return ctx.Status(409).JSON(map[string]string{"status": "error", "message": err.Error()})
		}
		return ctx.Status(400).JSON(map[string]string{"status": "error", "message": err.Error()})
	}
	return ctx.Status(201).JSON(map[string]string{
		"status": "success",
	})
}

func (h *RestHandler) Delete(ctx *fiber.Ctx) error {
	uuid := ctx.Params("uuid")
	if err := h.service.Delete(uuid); err != nil {
		if err.Error() == "uuid not found" {
			return ctx.Status(409).JSON(map[string]string{"status": "error", "message": err.Error()})
		}
		return ctx.Status(400).JSON(map[string]string{"status": "error", "message": err.Error()})
	}
	return ctx.Status(201).JSON(map[string]string{
		"status": "success",
	})
}

func (h *RestHandler) Put(ctx *fiber.Ctx) error {
	uuid := ctx.Params("uuid")
	user := &entity.UserEntity{}
	ctx.BodyParser(user)
	if err := h.service.Update(uuid, *user); err != nil {
		if err.Error() == "uuid not found" {
			return ctx.Status(409).JSON(map[string]string{"status": "error", "message": err.Error()})
		}
		return ctx.Status(400).JSON(map[string]string{"status": "error", "message": err.Error()})
	}
	return ctx.Status(201).JSON(map[string]string{
		"status": "success",
	})
}

func (h *RestHandler) Patch(ctx *fiber.Ctx) error {
	uuid := ctx.Params("uuid")
	user := &entity.UserEntity{}
	ctx.BodyParser(user)
	if err := h.service.Patch(uuid, *user); err != nil {
		if err.Error() == "uuid not found" {
			return ctx.Status(409).JSON(map[string]string{"status": "error", "message": err.Error()})
		}
		return ctx.Status(400).JSON(map[string]string{"status": "error", "message": err.Error()})
	}
	return ctx.Status(201).JSON(map[string]string{
		"status": "success",
	})
}
