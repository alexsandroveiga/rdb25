package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexsandroveiga/rdb25/src/configuration/database/redis"
	"github.com/alexsandroveiga/rdb25/src/configuration/queue"
	"github.com/alexsandroveiga/rdb25/src/domain"
	"github.com/alexsandroveiga/rdb25/src/repository"
	"github.com/alexsandroveiga/rdb25/src/worker"
	"github.com/gofiber/fiber/v3"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	client, err := redis.NewRedisConnection(context.Background())
	if err != nil {
		log.Fatalf("Error trying to connect to database, error=%s \n", err.Error())
		return
	}
	queue.Queue = make(chan domain.PaymentRequest, 10000)

	// queue := messaging.NewPaymentMessaging(client)
	repository := repository.NewRedisPaymentRepository(client)
	// go worker.StartWorker(queue, repository)
	worker.ProcessPayment(repository)
	app := fiber.New(fiber.Config{
		CaseSensitive: true,
		StrictRouting: true,
	})

	app.Get("/payments-summary", func(c fiber.Ctx) error {
		fromStr := c.Query("from")
		toStr := c.Query("to")
		var from time.Time
		var to time.Time
		var err error
		if fromStr == "" {
			from = time.Time{} // zero time, início do tempo Go
		} else {
			from, err = time.Parse(time.RFC3339, fromStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid from datetime"})
			}
		}
		if toStr == "" {
			to = time.Now().UTC()
		} else {
			to, err = time.Parse(time.RFC3339, toStr)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid to datetime"})
			}
		}
		summary, err := repository.GetSummary(from, to)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(http.StatusOK).JSON(summary)
	})

	app.Post("/payments", func(c fiber.Ctx) error {
		var req domain.PaymentRequest
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		select {
		case queue.Queue <- req:
			return c.SendStatus(fiber.StatusNoContent)
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "full queue"})
		}
		// if err := queue.Produce(context.Background(), req); err != nil {
		// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "full queue"})
		// }
		// return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/purge-payments", func(c fiber.Ctx) error {
		if err := repository.Purge(); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	log.Fatal(app.Listen(os.Getenv("PORT"), fiber.ListenConfig{EnablePrefork: true}))
}
