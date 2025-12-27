.PHONY: all build test clean proto run lint deps

# Build variables
BINARY_NAME=router
MAIN_PACKAGE=./cmd/router
PROTO_DIR=api/proto
PROTO_OUT=api/proto/routerpb

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

all: deps proto build

# Build the binary
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) $(MAIN_PACKAGE)

# Run tests
test:
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Generate protobuf code
proto:
	@mkdir -p $(PROTO_OUT)
	protoc --go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		-I$(PROTO_DIR) $(PROTO_DIR)/*.proto

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install protobuf tools
proto-tools:
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Run the router
run: build
	./bin/$(BINARY_NAME)

# Run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev:
	air

# Lint the code
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GOCMD) fmt ./...

# Build Docker image
docker-build:
	docker build -t peregrine-router:latest .

# Run locally with Ollama
run-local: build
	OLLAMA_ENDPOINT=http://localhost:11434 ./bin/$(BINARY_NAME)

# Generate mocks for testing
mocks:
	mockgen -source=internal/provider/provider.go -destination=internal/provider/mock_provider.go -package=provider

.DEFAULT_GOAL := all
