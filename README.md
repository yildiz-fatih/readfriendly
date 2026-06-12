# readfriendly

ReadFriendly is a web clipper built with Go, PostgreSQL, RabbitMQ and S3.

Give it a URL, it gets rid of the noise, you get a readable PDF or EPUB, optionally emailed to your inbox or e-reader.

## Quick Start

```bash
cp .env.example .env

make up # runs on http://localhost:8080
```

## Development

```bash
cp .env.example .env

make dev

make run-api # runs on http://localhost:8080
make run-worker
```
