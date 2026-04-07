BINARY  := certbot-go
BUILD   := build
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build workflow-build test run run-daemon init deps clean

build:
	@go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY) ./cmd/certbot-go

workflow-build:
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(BUILD)/$(BINARY)-$(GOOS)_$(GOARCH) ./cmd/certbot-go

test:
	@go test -v ./...

# 单次运行模式
run:
	@go run ./cmd/certbot-go --config config.yaml --once

# 守护进程模式
run-daemon:
	@go run ./cmd/certbot-go --config config.yaml

init:
	@if [ ! -f config.yaml ]; then \
		cp deploy/config.example.yaml config.yaml; \
		echo "config.yaml created from example, please edit it."; \
	else \
		echo "config.yaml already exists, skipping."; \
	fi
	@if [ ! -f .env ]; then \
		cp deploy/.env.example .env; \
		echo ".env created from example, please edit it with your AliDNS AccessKey id / secret"; \
	fi

deps:
	@go mod tidy

clean:
	@rm -rf $(BUILD)
	@go clean -cache
