GO ?= go
export GOTOOLCHAIN := local
export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod

.PHONY: run test vet build fmt modelcheck verify-modules smoke
run:
	$(GO) run ./cmd/server

test:
	$(GO) test -race -timeout=45s -coverprofile=coverage.out ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build -o bin/server ./cmd/server
	$(GO) build -o bin/modelcheck ./cmd/modelcheck

fmt:
	$(GO) fmt ./...

modelcheck:
	$(GO) run ./cmd/modelcheck

verify-modules:
	$(GO) mod verify

smoke: build
	python3 scripts/smoke.py
