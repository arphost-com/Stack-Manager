.PHONY: build dev test clean prepare-state

# Build everything
build:
	cd server && go build -o ../bin/stack-manager-server ./cmd/server
	cd web && npm ci && npm run build

# Run server locally
dev-server:
	cd server && go run ./cmd/server

# Run web dev server
dev-web:
	cd web && npm run dev

# Run tests
test:
	cd server && go test ./...
	cd web && npm test
	bash -n stack-manager.sh

# Cross-compile server for Linux
build-linux:
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/stack-manager-server-linux-amd64 ./cmd/server

# Docker
# Stamp the UI footer version with the current commit SHA (runs on the host,
# where git is available; the Docker build context has no .git).
VITE_GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null)
export VITE_GIT_SHA

prepare-state:
	./scripts/prepare-state.sh .env

docker-build:
	VITE_GIT_SHA=$(VITE_GIT_SHA) docker compose build

docker-up: prepare-state
	VITE_GIT_SHA=$(VITE_GIT_SHA) docker compose --env-file .env up -d --build

docker-down:
	docker compose down

# Clean
clean:
	rm -rf bin/ web/dist/ web/node_modules/
