package main

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/koalachatapp/user/cmd/rest/handler"
	"github.com/koalachatapp/user/internal/core/port"
	"github.com/koalachatapp/user/internal/core/service"
	"github.com/koalachatapp/user/internal/repository"
)

func main() {
	runtime.GOMAXPROCS(2)
	// repository
	userrepo := repository.NewUserRepository()

	// worker
	worker := port.Worker{
		Wg:     sync.WaitGroup{},
		Worker: make(chan func() error),
	}
	defer worker.Wg.Wait()

	// service
	userservice := service.NewUserService(userrepo, &worker)

	// handler
	userhandler := handler.NewRestHandler(userservice)

	app := fiber.New(fiber.Config{
		Prefork:           true,
		CaseSensitive:     true,
		UnescapePath:      true,
		ReduceMemoryUsage: true,
		JSONEncoder:       sonic.Marshal,
		JSONDecoder:       sonic.Unmarshal,
	})
	app.Use(recover.New())
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	app.Use(logger.New(logger.Config{
		TimeZone:     "Asian/Jakarta",
		TimeInterval: time.Millisecond,
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "127.0.0.1," + os.Getenv("ALLOWED_HOSTS"),
		AllowHeaders: "Origin,Content-Type,Accept",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE",
	}))
	app.Use(limiter.New(limiter.Config{
		Expiration: 5 * time.Second,
		Max:        100,
		LimitReached: func(c *fiber.Ctx) error {
			return c.JSON(map[string]string{"status": "error", "message": "too fast"})
		},
	}))
	app.Use("/", func(ctx *fiber.Ctx) error {
		head := ctx.GetReqHeaders()
		if head["Token"] == "" {
			return ctx.Status(401).JSON(map[string]string{"status": "error", "message": "Not Authorized"})
		}
		// FUTURE: call from auth server for validate token
		if head["Token"] != "koala" {
			return ctx.Status(401).JSON(map[string]string{"status": "error", "message": "Invalid Authorization"})
		}

		return ctx.Next()
	})
	if os.Getenv("ENV") == "dev" {
		app.Get("/monitor", monitor.New(monitor.Config{Refresh: 1 * time.Second}))
	}
	app.Post("/register", userhandler.Post)
	app.Delete("/remove/:uuid", userhandler.Delete)
	app.Put("/update/:uuid", userhandler.Put)
	app.Patch("/patch/:uuid", userhandler.Patch)
	app.Listen(":3030")

}
