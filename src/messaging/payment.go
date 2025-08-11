package messaging

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alexsandroveiga/rdb25/src/domain"
	"github.com/redis/go-redis/v9"
)

func NewPaymentMessaging(client *redis.Client) PaymentMessaging {
	return &paymentMessaging{client}
}

type paymentMessaging struct {
	client *redis.Client
}

type PaymentMessaging interface {
	Produce(ctx context.Context, p domain.PaymentRequest) error
	Consume(ctx context.Context) (domain.PaymentRequest, error)
}

func (pm *paymentMessaging) Produce(ctx context.Context, p domain.PaymentRequest) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return pm.client.RPush(ctx, "payment_queue", data).Err()
}

func (m *paymentMessaging) Consume(ctx context.Context) (domain.PaymentRequest, error) {
	res, err := m.client.BLPop(ctx, 0*time.Second, "payment_queue").Result()
	if err != nil {
		return domain.PaymentRequest{}, err
	}

	var p domain.PaymentRequest
	if err := json.Unmarshal([]byte(res[1]), &p); err != nil {
		return domain.PaymentRequest{}, err
	}

	return p, nil
}
