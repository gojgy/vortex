.PHONY: run build test clean cluster-up cluster-down

STATIC_ROOT := $(shell pwd)/web/static

run:
	@echo ">> Starting Vortex locally..."
	@echo ">> Using static root: $(STATIC_ROOT)"
	@VORTEX_STATIC_ROOT=$(STATIC_ROOT) \
	VORTEX_CONFIG_FILE_PATH=./ \
	VORTEX_CONFIG_FILE_NAME=vortex \
	VORTEX_APP_ENVIRONMENT=dev \
	go run ./cmd/vortex/

build:
	@echo ">> Building Vortex binary..."
	@go build -o ./bin/vortex ./cmd/vortex/

test:
	@echo ">> Running all tests..."
	@go test -v -race ./...

clean:
	@echo ">> Cleaning up..."
	@rm -f ./bin/vortex
