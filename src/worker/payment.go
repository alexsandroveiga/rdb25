package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexsandroveiga/rdb25/src/configuration/queue"
	"github.com/alexsandroveiga/rdb25/src/domain"
	"github.com/alexsandroveiga/rdb25/src/messaging"
	"github.com/alexsandroveiga/rdb25/src/repository"
	"github.com/alexsandroveiga/rdb25/src/util"
)

var (
	WorkerCount = 4
)

func ProcessPayment(repository repository.RedisPaymentRepository) {
	client := &http.Client{}
	var firstDefaultFail bool = true
	for i := range WorkerCount {
		go func(id int) {
			for req := range queue.Queue {
				var processor string
				now := time.Now().UTC()
				req.RequestedAt = now.Format("2006-01-02T15:04:05.999Z")

				if util.IsHealthy("default") && sendToProcessor(client, os.Getenv("URL_PROCESSOR_DEFAULT"), req) {
					processor = "default"
				} else {
					if firstDefaultFail {
						log.Println("⏳ Aguardando 3s antes de tentar fallback (primeira falha do default)")
						time.Sleep(3 * time.Second)
						firstDefaultFail = false
					}
					if sendToProcessor(client, os.Getenv("URL_PROCESSOR_DEFAULT"), req) {
						processor = "default"
					} else if util.IsHealthy("fallback") && sendToProcessor(client, os.Getenv("URL_PROCESSOR_FALLBACK"), req) {
						processor = "fallback"
					}
				}

				// if util.IsHealthy("default") {
				// 	if sendToProcessor(client, "http://localhost:8001/payments", req) {
				// 		processor = "default"
				// 	} else if util.IsHealthy("fallback") && sendToProcessor(client, "http://localhost:8002/payments", req) {
				// 		processor = "fallback"
				// 	}
				// } else if util.IsHealthy("fallback") {
				// 	if sendToProcessor(client, "http://localhost:8002/payments", req) {
				// 		processor = "fallback"
				// 	} else if util.IsHealthy("default") && sendToProcessor(client, "http://localhost:8001/payments", req) {
				// 		processor = "default"
				// 	}
				// } else {
				// 	if sendToProcessor(client, "http://localhost:8001/payments", req) {
				// 		processor = "default"
				// 	} else if util.IsHealthy("fallback") && sendToProcessor(client, "http://localhost:8002/payments", req) {
				// 		processor = "fallback"
				// 	}
				// }
				if processor == "" {
					// log.Printf("⚠ Nenhum processor disponível para %s", req.CorrelationID)

					go func(r domain.PaymentRequest) {
						time.Sleep(200 * time.Millisecond)
						select {
						case queue.Queue <- r:
							// log.Printf("♻ Reenfileirado: %s", r.CorrelationID)
						default:
							// log.Printf("❌ Fila cheia, não foi possível reenfileirar: %s", r.CorrelationID)
						}
					}(req)

					continue
				}
				// if processor == "fallback" {
				// 	log.Printf("🔴 Salvo no fallback %s", req.CorrelationID)
				// }
				p := domain.Payment{
					CorrelationID: req.CorrelationID,
					Amount:        req.Amount,
					RequestedAt:   now,
					Processor:     processor,
				}
				repository.Process(p)
			}
		}(i)
	}
}

func StartWorker(queue messaging.PaymentMessaging, repository repository.RedisPaymentRepository) {
	ctx := context.Background()
	client := &http.Client{}
	var firstDefaultFail bool = true

	for {
		req, err := queue.Consume(ctx) // Bloqueia até ter mensagem
		if err != nil {
			log.Println("Erro ao consumir:", err)
			continue
		}
		var processor string
		now := time.Now().UTC()
		req.RequestedAt = now.Format("2006-01-02T15:04:05.999Z")
		if util.IsHealthy("default") && sendToProcessor(client, os.Getenv("URL_PROCESSOR_DEFAULT"), req) {
			processor = "default"
		} else {
			if firstDefaultFail {
				log.Println("⏳ Aguardando 5s antes de tentar fallback (primeira falha do default)")
				time.Sleep(5 * time.Second)
				firstDefaultFail = false
			}
			if sendToProcessor(client, os.Getenv("URL_PROCESSOR_DEFAULT"), req) {
				processor = "default"
			} else if util.IsHealthy("fallback") && sendToProcessor(client, os.Getenv("URL_PROCESSOR_FALLBACK"), req) {
				processor = "fallback"
			}
		}
		if processor == "" {
			// log.Printf("⚠ Nenhum processor disponível para %s", req.CorrelationID)

			go func(r domain.PaymentRequest) {
				time.Sleep(200 * time.Millisecond)
				log.Printf("♻ Reenfileirado: %s", r.CorrelationID)
				if err := queue.Produce(ctx, req); err != nil {
					log.Printf("❌ Fila cheia, não foi possível reenfileirar: %s", r.CorrelationID)
				}
			}(req)

			continue
		}
		if processor == "fallback" {
			log.Printf("🔴 Salvo no fallback %s", req.CorrelationID)
		}
		p := domain.Payment{
			CorrelationID: req.CorrelationID,
			Amount:        req.Amount,
			RequestedAt:   now,
			Processor:     processor,
		}
		repository.Process(p)
	}
}

func sendToProcessor(client *http.Client, url string, req domain.PaymentRequest) bool {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return false
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return false
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		log.Printf("🔴🔴🔴 ERRO 4XX => %d 🔴🔴🔴", resp.StatusCode)
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
