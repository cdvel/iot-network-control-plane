.PHONY: proto build run test docker clean

# Generate protobuf code
proto:
	./scripts/proto-gen.sh

# Build binaries
build: proto
	go build -o bin/server cmd/server/main.go
	go build -o bin/device cmd/device/main.go
	go build -o bin/cli cmd/cli/main.go

# Run locally
run-server:
	go run cmd/server/main.go

run-device:
	go run cmd/device/main.go -server localhost:50051 -type router

# Tests
test:
	go test -v -cover ./...

test-integration:
	go test -v ./test/integration/...

# Docker
docker-build:
	docker build -t network-control-plane-server:latest -f Dockerfile.server .
	docker build -t network-control-plane-device:latest -f Dockerfile.device .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Demo
demo: docker-build docker-up
	@echo "Control plane running on:"
	@echo "  gRPC: localhost:50051"
	@echo "  REST: http://localhost:8080"
	@echo "  Prometheus: http://localhost:9090"
	@sleep 5
	@echo "\nDevices registered:"
	@curl -s http://localhost:8080/api/v1/devices | jq .

# Query devices
query:
	@echo "All devices:"
	@curl -s http://localhost:8080/api/v1/devices | jq .
	@echo "\nStats:"
	@curl -s http://localhost:8080/api/v1/stats | jq .

# Clean
clean:
	rm -rf bin/
	docker-compose down -v
	rm -f api/proto/*.pb.go

# Development setup
dev-setup:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go mod download