package handler

import "github.com/gofiber/fiber/v2"

type RestHandler struct{}

func NewRestHandler() *RestHandler {
	return &RestHandler{}
}

func (h *RestHandler) Post(ctx *fiber.Ctx) error {
	return nil
}
