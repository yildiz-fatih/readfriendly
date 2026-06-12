up:
	docker compose up -d

dev:
	docker compose up -d db migrate rabbitmq

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker
