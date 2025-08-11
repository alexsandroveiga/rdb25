package repository

import (
	"time"

	"github.com/alexsandroveiga/rdb25/src/configuration/storage"
	"github.com/alexsandroveiga/rdb25/src/domain"
)

func NewPaymentRepository(storage *storage.InMemoryStorage) PaymentRepository {
	return &paymentRepository{storage}
}

type paymentRepository struct {
	storage *storage.InMemoryStorage
}

type PaymentRepository interface {
	Process(p domain.Payment)
	GetSummary(from, to time.Time) domain.PaymentSummary
}

func (pr *paymentRepository) Process(p domain.Payment) {
	pr.storage.InsertOne("payments", p)
}

func (pr *paymentRepository) GetSummary(from, to time.Time) domain.PaymentSummary {
	data := pr.storage.Get("payments")
	var summary domain.PaymentSummary
	for _, item := range data {
		p, ok := item.(domain.Payment)
		if !ok {
			continue
		}
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
