up:
	docker compose up -d

dev:
	docker compose up -d db migrate rabbitmq gotenberg pandoc

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docs:
	swag init -g cmd/api/main.go --parseInternal
