package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/koalachatapp/user/cmd/rest/handler"
	"github.com/koalachatapp/user/internal/core/service"
	"github.com/koalachatapp/user/internal/repository"
)

func main() {
	userrepo := repository.NewUserRepository()
	// service
	userservice := service.NewUserService(userrepo)

	// handler
	userhandler := handler.NewRestHandler(userservice)

	app := fiber.New()
	app.Use(logger.New(logger.Config{
		TimeZone:     "Asian/Jakarta",
		TimeInterval: time.Millisecond,
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin,Content-Type,Accept",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE",
	}))
	app.Post("/register", userhandler.Post)
	app.Delete("/remove/:uuid", userhandler.Delete)
	app.Put("/update/:uuid", userhandler.Put)
	app.Patch("/update/:uuid", userhandler.Patch)
	app.Listen(":3030")

}
