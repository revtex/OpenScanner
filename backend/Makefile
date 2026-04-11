# Backend Makefile

.PHONY: build dev test lint clean migrate generate

BINARY=openscanner
CMD=./cmd/server

build:
	go build -o $(BINARY) $(CMD)

dev:
	air

test:
	go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) server migrate
	rm -rf tmp/
	rm -f openscanner.db openscanner.db-wal openscanner.db-shm

migrate:
	go run ./cmd/migrate up

generate:
	cd sqlc && sqlc generate
