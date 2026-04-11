# OpenScanner — Root Makefile
# Delegates to backend and frontend sub-makes

.PHONY: all build dev test lint clean migrate generate

all: build

build:
	$(MAKE) -C backend build
	$(MAKE) -C frontend build

dev:
	$(MAKE) -C backend dev &
	$(MAKE) -C frontend dev

test:
	$(MAKE) -C backend test
	$(MAKE) -C frontend test

lint:
	$(MAKE) -C backend lint
	$(MAKE) -C frontend lint

migrate:
	$(MAKE) -C backend migrate

generate:
	$(MAKE) -C backend generate

clean:
	$(MAKE) -C backend clean
	$(MAKE) -C frontend clean
