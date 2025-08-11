package domain

import "time"

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
