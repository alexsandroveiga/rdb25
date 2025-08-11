package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthStatus struct {
	Failing         bool  `json:"failing"`
	MinResponseTime int64 `json:"minResponseTime"`
}

var (
	cache      = make(map[string]HealthStatus)
	lastCheck  = make(map[string]time.Time)
	cacheMutex sync.Mutex
)

func IsHealthy(processor string) bool {
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

func fetchHealth(processor string) HealthStatus {
	url := map[string]string{
		"default":  "http://localhost:8001/payments/service-health",
		"fallback": "http://localhost:8002/payments/service-health",
	}[processor]

	resp, err := http.Get(url)
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
