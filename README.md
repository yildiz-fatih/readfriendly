# readfriendly

ReadFriendly is a web clipper built with Go, PostgreSQL, RabbitMQ and S3.

Give it a URL, it gets rid of the noise, you get a readable PDF or EPUB, optionally emailed to your inbox or e-reader.


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
