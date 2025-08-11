package util

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	client      = &http.Client{Timeout: 5 * time.Second}
	cache       = make(map[string]HealthStatus)
	lastCheck   = make(map[string]time.Time)
	cacheMutex  sync.RWMutex
	ctx         = context.Background()
	redisClient = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})
)

type HealthStatus struct {
	Failing         bool  `json:"failing"`
	MinResponseTime int64 `json:"minResponseTime"`
}

func IsHealthyInMemory(processor string) bool {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	now := time.Now()
	last, ok := lastCheck[processor]
	if !ok || now.Sub(last) > 5*time.Second {
		status := fetchHealth(processor)
		cache[processor] = status
		lastCheck[processor] = now
	}
	return !cache[processor].Failing
}

func IsHealthy(processor string) bool {
	cacheKey := "health_status:" + processor
	lastCheckKey := "health_last_check:" + processor
	lastCheckStr, err := redisClient.Get(ctx, lastCheckKey).Result()
	var lastCheck time.Time
	if err == nil {
		lastCheck, _ = time.Parse(time.RFC3339Nano, lastCheckStr)
	}
	now := time.Now()

	if err != nil || now.Sub(lastCheck) > 5*time.Second {
		// Faz o fetch e salva cache no Redis
		status := fetchHealth(processor)

		statusJSON, err := json.Marshal(status)
		if err == nil {
			redisClient.Set(ctx, cacheKey, statusJSON, 0)
		}
		redisClient.Set(ctx, lastCheckKey, now.Format(time.RFC3339Nano), 0)

		return !status.Failing
	}

	// Busca do cache JSON no Redis
	statusJSON, err := redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		// Se cache não existir, força fetchHealth
		status := fetchHealth(processor)
		return !status.Failing
	}

	var cachedStatus HealthStatus
	if err := json.Unmarshal([]byte(statusJSON), &cachedStatus); err != nil {
		// Se erro no unmarshal, força fetchHealth
		status := fetchHealth(processor)
		return !status.Failing
	}

	return !cachedStatus.Failing
}

func fetchHealth(processor string) HealthStatus {
	url := map[string]string{
		"default":  os.Getenv("URL_HEALTH_DEFAULT"),
		"fallback": os.Getenv("URL_HEALTH_FALLBACK"),
	}[processor]
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Health check error for %s: %v", processor, err)
		return HealthStatus{Failing: true}
	}
	defer resp.Body.Close()
	var status HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		log.Printf("Health check JSON error for %s", processor)
		return HealthStatus{Failing: true}
	}
	return status
}
