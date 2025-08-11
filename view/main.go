package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// ================== TYPES ==================

type Payment struct {
	CorrelationID string    `json:"correlationId"`
	Amount        float64   `json:"amount"`
	RequestedAt   time.Time `json:"requestedAt"`
	Processor     string    `json:"processor"`
}

type PaymentRequest struct {
	CorrelationID string  `json:"correlationId"`
	Amount        float64 `json:"amount"`
	RequestedAt   string  `json:"requestedAt"`
}

type PaymentSummary struct {
	Default  SummaryItem `json:"default"`
	Fallback SummaryItem `json:"fallback"`
}

type SummaryItem struct {
	TotalAmount   float64 `json:"totalAmount"`
	TotalRequests int     `json:"totalRequests"`
}

type HealthResponse struct {
	Failing         bool `json:"failing"`
	MinResponseTime int  `json:"minResponseTime"`
}

// ================== STORAGE ==================

type InMemoryStorage struct {
	mu   sync.RWMutex
	data []Payment
}

func NewInMemoryStorage(capacity int) *InMemoryStorage {
	return &InMemoryStorage{data: make([]Payment, 0, capacity)}
}

func (s *InMemoryStorage) Add(p Payment) {
	s.mu.Lock()
	s.data = append(s.data, p)
	s.mu.Unlock()
}

func (s *InMemoryStorage) Summary(from, to time.Time) PaymentSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary PaymentSummary
	for _, p := range s.data {
		if p.RequestedAt.Before(from) || p.RequestedAt.After(to) {
			continue
		}

		if p.Processor == "fallback" {
			summary.Fallback.TotalAmount += p.Amount
			summary.Fallback.TotalRequests++
		} else {
			summary.Default.TotalAmount += p.Amount
			summary.Default.TotalRequests++
		}
	}
	return summary
}

// ================== GLOBALS ==================

const (
	// QueueSize   = 10000
	// WorkerCount = 8
	QueueSize   = 10000
	WorkerCount = 8
)

var (
	queue   chan PaymentRequest
	storage = NewInMemoryStorage(QueueSize)
)

// ================== WORKERS ==================

func startWorkers() {
	// client := http.Client{Timeout: 2 * time.Second}
	client := http.Client{}

	for i := range WorkerCount {
		go func(id int) {
			for req := range queue {
				var processor string

				now := time.Now().UTC()
				req.RequestedAt = now.Format("2006-01-02T15:04:05.999Z")

				// Tenta default primeiro, se saudável
				if IsHealthy("default") {
					if sendToProcessor(client, "http://localhost:8001/payments", req) {
						processor = "default"
					} else if IsHealthy("fallback") && sendToProcessor(client, "http://localhost:8002/payments", req) {
						processor = "fallback"
					}
				} else if IsHealthy("fallback") {
					if sendToProcessor(client, "http://localhost:8002/payments", req) {
						processor = "fallback"
					} else if IsHealthy("default") && sendToProcessor(client, "http://localhost:8001/payments", req) {
						processor = "default"
					}
				} else {
					if sendToProcessor(client, "http://localhost:8001/payments", req) {
						processor = "default"
					} else if IsHealthy("fallback") && sendToProcessor(client, "http://localhost:8002/payments", req) {
						processor = "fallback"
					}
				}

				// if sendToProcessor(client, "http://localhost:8001/payments", req) {
				// 	processor = "default"
				// } else {
				// 	// Se falhou, tenta fallback somente se fallback estiver saudável
				// 	if IsHealthy("fallback") && sendToProcessor(client, "http://localhost:8002/payments", req) {
				// 		log.Printf("🟢 Usando fallback para %s", req.CorrelationID)
				// 		processor = "fallback"
				// 	} else {
				// 		// Última chance tenta default de novo (mesmo sem health saudável)
				// 		if sendToProcessor(client, "http://localhost:8001/payments", req) {
				// 			processor = "default"
				// 		}
				// 	}
				// }

				// Se nenhum processor processou, pula
				if processor == "" {
					log.Printf("⚠ Nenhum processor disponível para %s", req.CorrelationID)
					continue
				}

				// Salva pagamento processado
				p := Payment{
					CorrelationID: req.CorrelationID,
					Amount:        req.Amount,
					RequestedAt:   now,
					Processor:     processor,
				}
				storage.Add(p)
			}
		}(i)
	}
}

func sendToProcessor(client http.Client, url string, req PaymentRequest) bool {
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
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// ================== HANDLERS ==================

func handlePayments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	select {
	case queue <- req:
		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, "queue full", http.StatusServiceUnavailable)
	}
}

func handlePaymentsSummary(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	var from time.Time
	var to time.Time
	var err error

	if fromStr == "" {
		from = time.Time{} // zero time, início do tempo Go
	} else {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "invalid 'from' format, use RFC3339 e.g. 2020-07-10T12:34:56Z", http.StatusBadRequest)
			return
		}
	}

	if toStr == "" {
		to = time.Now().UTC()
	} else {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "invalid 'to' format, use RFC3339 e.g. 2020-07-10T12:34:56Z", http.StatusBadRequest)
			return
		}
	}

	summary := storage.Summary(from, to)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// ================== MAIN ==================

func main() {
	queue = make(chan PaymentRequest, QueueSize)
	startWorkers()

	mux := http.NewServeMux()
	mux.HandleFunc("/payments", handlePayments)
	mux.HandleFunc("/payments-summary", handlePaymentsSummary)

	log.Println("Listening on :9999")
	if err := http.ListenAndServe(":9999", mux); err != nil {
		log.Fatal(err)
	}
}
