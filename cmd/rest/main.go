package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/koalachatapp/user/cmd/rest/handler"
)

func main() {
	// handler
	handler := handler.NewRestHandler()

	app := fiber.New()
	app.Use(logger.New())
	app.Post("/signup", handler.Post)
	// app.Po
	app.Listen(":3030")

}
