package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	redis "github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/context"
)

type PaymentRequest struct {
	UserID         string  `json:"user_id"`
	Amount         float64 `json:"amount"`
	IdempotencyKey string  `json:"idempotency_key"`
}

type PaymentResponse struct {
	Message string `json:"message"`
}

var (
	ctx            = context.Background()
	rdb            *redis.Client
	rateLimiterMu  sync.Mutex
	tokens         = 10
	requestsTotal  = prometheus.NewCounter(prometheus.CounterOpts{Name: "payment_requests_total", Help: "Total number of payment requests received"})
	requestsFailed = prometheus.NewCounter(prometheus.CounterOpts{Name: "payment_requests_failed", Help: "Total number of failed payment requests"})
)

func init() {
	logDir := "./logs"
	_ = os.MkdirAll(logDir, 0755)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	//log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	logFile, err := os.OpenFile(logDir+"/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open log file")
	}
	log.Logger = zerolog.New(logFile).With().Timestamp().Logger()

	prometheus.MustRegister(requestsTotal, requestsFailed)

}

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		DB:   0,
	})
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal().Err(err).Msg("Redis connection failed")
	}
	log.Info().Msg("Connected to Redis")
}

func refillTokens() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		rateLimiterMu.Lock()
		tokens = 10
		rateLimiterMu.Unlock()
	}
}

func paymentHandler(w http.ResponseWriter, r *http.Request) {
	requestsTotal.Inc()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		requestsFailed.Inc()
		return
	}

	rateLimiterMu.Lock()
	if tokens <= 0 {
		rateLimiterMu.Unlock()
		http.Error(w, "Too many requests, please retry later", http.StatusTooManyRequests)
		requestsFailed.Inc()
		return
	}
	tokens--
	rateLimiterMu.Unlock()

	var req PaymentRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		requestsFailed.Inc()
		return
	}

	key := "payment:idempotency:" + req.IdempotencyKey
	exists, err := rdb.Exists(ctx, key).Result()
	if err == nil && exists == 1 {
		if err := json.NewEncoder(w).Encode(PaymentResponse{Message: "Payment already processed"}); err != nil {
			log.Error().Err(err).Msg("Failed to encode payment response")
		}
		log.Info().Str("key", req.IdempotencyKey).Msg("Duplicate payment request")
		return
	}

	if rand.Intn(10) < 3 {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Error().Str("key", req.IdempotencyKey).Msg("Simulated internal error")
		requestsFailed.Inc()
		return
	}

	rdb.Set(ctx, key, "processed", 24*time.Hour)
	if err := json.NewEncoder(w).Encode(PaymentResponse{Message: "Payment already processed"}); err != nil {
		log.Error().Err(err).Msg("Failed to encode payment response")
	}
	log.Info().Str("key", req.IdempotencyKey).Msg("Payment processed")
}

func retryWithExponentialBackoff(url string, request PaymentRequest) {
	maxRetries := 5
	delay := 500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		body, _ := json.Marshal(request)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
		if err == nil && resp.StatusCode == http.StatusOK {
			log.Info().Str("key", request.IdempotencyKey).Msg("Retry succeeded")
			return
		} else {
			if resp != nil {
				log.Warn().Str("key", request.IdempotencyKey).Int("retry", i+1).Str("status", resp.Status).Msg("Retry failed")
			} else {
				log.Warn().Str("key", request.IdempotencyKey).Int("retry", i+1).Err(err).Msg("Retry error")
			}
			time.Sleep(delay)
			delay *= 2
		}
	}
	log.Error().Str("key", request.IdempotencyKey).Msg("All retries failed")
}

func simulateClients() {
	for i := 0; i < 5; i++ {
		go func(i int) {
			key := "key-" + strconv.Itoa(i)
			request := PaymentRequest{
				UserID:         "user" + strconv.Itoa(i),
				Amount:         100.0,
				IdempotencyKey: key,
			}
			retryWithExponentialBackoff("http://localhost:8123/pay", request)
		}(i)
	}
}

func main() {
	initRedis()
	go refillTokens()
	go simulateClients()

	http.HandleFunc("/pay", paymentHandler)
	http.Handle("/metrics", promhttp.Handler())

	log.Info().Msg("Starting server on :8123")
	if err := http.ListenAndServe(":8123", nil); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
