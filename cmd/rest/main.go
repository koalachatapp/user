package main

import (
	"log"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/Shopify/sarama"
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
	rand.Seed(time.Now().Unix())
	// repository
	userrepo := repository.NewUserRepository()

	saramaconfig := sarama.NewConfig()
	saramaconfig.Producer.Return.Successes = true
	saramaconfig.Net.MaxOpenRequests = 5
	saramaddr := os.Getenv("KAFKA_URL")
	if saramaddr == "" {
		saramaddr = "kafka:9092"
	}
	prod, err := sarama.NewSyncProducer([]string{saramaddr}, saramaconfig)
	if err != nil {
		log.Println(err)
	} else {
		defer prod.Close()
	}

	// worker
	worker := port.Worker{
		Wg:         sync.WaitGroup{},
		Wg2:        sync.WaitGroup{},
		Worker:     make(chan map[uint8]interface{}),
		Prod:       prod,
		Lock:       sync.Mutex{},
		Send2kafka: make(chan string, 256),
	}
	defer worker.Wg.Wait()
	defer worker.Wg2.Wait()
	// go worker.RunProduserWorker()

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
	app.Use("/register", userhandler.TokenValidate)
	app.Use("/remove", userhandler.TokenValidate)
	app.Use("/update", userhandler.TokenValidate)
	app.Use("/patch", userhandler.TokenValidate)
	if os.Getenv("ENV") == "dev" {
		app.Get("/monitor", monitor.New(monitor.Config{Refresh: 1 * time.Second}))
	}
	app.Post("/register", userhandler.Post)
	app.Delete("/remove/:uuid", userhandler.Delete)
	app.Put("/update/:uuid", userhandler.Put)
	app.Patch("/patch/:uuid", userhandler.Patch)
	app.Listen(":3030")

}
