.PHONY: build dev test clean prepare-state

# Build everything
build:
	cd server && go build -o ../bin/compose-manager-server ./cmd/server
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
	bash -n compose-manager.sh

# Cross-compile server for Linux
build-linux:
	cd server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../bin/compose-manager-server-linux-amd64 ./cmd/server

# Docker
prepare-state:
	./scripts/prepare-state.sh .env

docker-build:
	docker compose build

docker-up: prepare-state
	docker compose --env-file .env up -d --build

docker-down:
	docker compose down

# Clean
clean:
	rm -rf bin/ web/dist/ web/node_modules/
