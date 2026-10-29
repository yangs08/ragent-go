GO ?= go
export GOTOOLCHAIN := local
export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod

.PHONY: run test vet build fmt modelcheck verify-modules smoke agentcheck
run:
	$(GO) run ./cmd/server

test:
	$(GO) test -race -timeout=45s -coverprofile=coverage.out ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -o bin/server ./cmd/server
	$(GO) build -o bin/modelcheck ./cmd/modelcheck
	$(GO) build -o bin/agentcheck ./cmd/agentcheck
	$(GO) build -o bin/migrate ./cmd/migrate
	$(GO) build -o bin/dbcheck ./cmd/dbcheck

fmt:
	$(GO) fmt ./...

modelcheck:
	$(GO) run ./cmd/modelcheck

verify-modules:
	$(GO) mod verify

smoke: build
	python3 scripts/smoke.py

agentcheck:
	$(GO) run ./cmd/agentcheck

.PHONY: db-up db-stop migrate migrate-status dbcheck
db-up:
	docker compose --env-file .env -f deploy/compose.yaml up -d --wait postgres

db-stop:
	docker compose --env-file .env -f deploy/compose.yaml stop postgres

migrate:
	$(GO) run ./cmd/migrate -command up

migrate-status:
	$(GO) run ./cmd/migrate -command status

dbcheck:
	$(GO) run ./cmd/dbcheck
