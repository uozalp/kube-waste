BINARY := kube-waste
BIN_DIR := bin
PKG := ./cmd/kube-waste

GO ?= go
GOFLAGS ?=

.PHONY: all build run test test-race cover fmt vet check tidy clean

all: check build

build:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) $(PKG)

run:
	$(GO) run $(PKG) $(ARGS)

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

check: fmt vet test

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR) coverage.out
