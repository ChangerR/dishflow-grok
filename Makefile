.PHONY: up down migrate serve worker test vet lint-openapi admin-dev check

up:
	docker compose up -d

down:
	docker compose down

migrate:
	go run ./cmd/dishflow migrate

serve:
	go run ./cmd/dishflow serve

worker:
	go run ./cmd/dishflow worker

healthcheck:
	go run ./cmd/dishflow healthcheck

test:
	go test ./... -race -count=1

vet:
	go vet ./...

admin-dev:
	pnpm --filter @dishflow/admin dev

check:
	pnpm --recursive run check
	pnpm --recursive run test
	pnpm --recursive run build
