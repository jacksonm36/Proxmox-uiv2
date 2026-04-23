.PHONY: all api worker web test tidy migrate install

all: api

# One-shot: native Postgres (if sudo works), .env, build dist/* and apps/web/dist
install:
	@chmod +x scripts/install.sh install 2>/dev/null; ./scripts/install.sh

api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

web:
	cd apps/web && npm run dev

tidy:
	go mod tidy

test:
	go test ./...

# Migrations are embedded; `go run ./cmd/api` runs them on start. For manual goose, point at internal/migrate/migrations.
migrate:
	@echo "Use embedded migration on API startup, or: goose -dir internal/migrate/migrations postgres \"$$CM_DATABASE_URL\" up"
