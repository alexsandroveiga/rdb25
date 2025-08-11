package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexsandroveiga/rdb25/src/domain"
	"github.com/redis/go-redis/v9"
)

func NewRedisPaymentRepository(client *redis.Client) RedisPaymentRepository {
	return &redisPaymentRepository{client}
}

type RedisPaymentRepository interface {
	Process(p domain.Payment) error
	GetSummary(from, to time.Time) (domain.PaymentSummary, error)
	Purge() error
}

type redisPaymentRepository struct {
	client *redis.Client
}

func (r *redisPaymentRepository) GetSummary(from time.Time, to time.Time) (domain.PaymentSummary, error) {
	var summary domain.PaymentSummary
	ctx := context.Background()
	iter := r.client.Scan(ctx, 0, "payment:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		val, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var p domain.Payment
		if err := json.Unmarshal([]byte(val), &p); err != nil {
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
	if err := iter.Err(); err != nil {
		return domain.PaymentSummary{}, err
	}
	return summary, nil
}

func (r *redisPaymentRepository) Process(p domain.Payment) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return r.client.Set(context.Background(), fmt.Sprintf("payment:%s", p.CorrelationID), data, 0).Err()
}

func (r *redisPaymentRepository) Purge() error {
	ctx := context.Background()
	iter := r.client.Scan(ctx, 0, "payment:*", 0).Iterator()
	for iter.Next(ctx) {
		if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
