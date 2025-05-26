# Idempotent Payment API

A resilient, idempotent payment API built in Go with Redis for idempotency guarantees, Prometheus metrics, structured logging, exponential backoff retries, and rate limiting to handle thundering herd scenarios.

## Features

* **Idempotent Payments** using Redis to avoid double processing.
* **Exponential Backoff** retries for transient failures.
* **Rate Limiting** via token bucket algorithm.
* **Structured Logging** using `zerolog`.
* **Prometheus Metrics** exported at `/metrics`.
* **Client Simulation** to test retries and concurrency.

## Requirements

* Go 1.21+
* Docker
* Redis (running locally or via Docker)

## Getting Started

### 1. Clone the Repository

```bash
git clone https://github.com/your-username/idempotent-payment-api.git
cd idempotent-payment-api
```

### 2. Start Redis

```bash
docker run --name redis -p 6379:6379 -d redis
```

### 3. Build and Run the Application

```bash
docker build -t idempotent-payment-api .
docker run -p 8080:8080 --network host idempotent-payment-api
```

Alternatively, run it locally:

```bash
go run main.go
```

### 4. Simulate Clients

Clients are automatically simulated inside the app using goroutines. They will attempt payment with retries.

## API

### `POST /pay`

Processes a payment.

**Request Body:**

```json
{
  "user_id": "user123",
  "amount": 100.0,
  "idempotency_key": "unique-key-123"
}
```

**Responses:**

* `200 OK`: Payment processed or already processed
* `429 Too Many Requests`: Rate limit exceeded
* `500 Internal Server Error`: Random simulated failure (for retry testing)

### `GET /metrics`

Prometheus metrics endpoint.

## Metrics

* `payment_requests_total`: Count of total requests.
* `payment_requests_failed`: Count of failed requests.

## Logging

Structured logs printed to the console using `zerolog`. Example:

```
{"time":1684786930,"level":"info","message":"Payment processed","key":"key-0"}
```

## License

MIT

---

📬 Questions or improvements? Feel free to open an issue or PR!
