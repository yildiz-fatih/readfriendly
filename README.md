# readfriendly

built with Go, PostgreSQL, RabbitMQ and S3.

## Quick Start

```bash
cp .env.example .env

docker compose up # runs on http://localhost:8080
```

## Development

```bash
cp .env.example .env

docker compose up db migrate rabbitmq

go run ./cmd/api # runs on http://localhost:8080
go run ./cmd/worker
```
