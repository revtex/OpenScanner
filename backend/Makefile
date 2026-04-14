# Backend Makefile

.PHONY: build dev test lint clean migrate generate

BINARY=openscanner
CMD=./cmd/server
OUTPUT?=$(BINARY)

build:
	swag init -g cmd/server/main.go --parseDependency --parseInternal -o docs
	go build -o $(OUTPUT) $(CMD)

dev:
	air

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) server migrate
	rm -rf tmp/
	rm -f docs/docs.go docs/swagger.json docs/swagger.yaml
	rm -f ../data/openscanner.db ../data/openscanner.db-wal ../data/openscanner.db-shm
	rm -rf ../data/recordings/

migrate:
	go run ./cmd/migrate up

generate:
	cd sqlc && sqlc generate
