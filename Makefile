# OpenScanner — Root Makefile
# Delegates to backend and frontend sub-makes

.PHONY: all build dev test lint clean migrate generate

EMBED_DIR=backend/internal/static/dist
BUILD_DIR=build
BACKEND_BINARY=$(BUILD_DIR)/openscanner

all: build

build:
	$(MAKE) -C frontend build
	rm -rf $(EMBED_DIR)/*
	cp -r frontend/dist/. $(EMBED_DIR)/
	mkdir -p $(BUILD_DIR)
	$(MAKE) -C backend build OUTPUT=../$(BACKEND_BINARY)

dev:
	$(MAKE) -C backend dev & \
	BACKEND_PID=$$!; \
	trap 'kill -- -$$BACKEND_PID 2>/dev/null || kill $$BACKEND_PID 2>/dev/null; pkill -P $$BACKEND_PID 2>/dev/null' EXIT; \
	$(MAKE) -C frontend dev; \
	kill -- -$$BACKEND_PID 2>/dev/null || kill $$BACKEND_PID 2>/dev/null; \
	pkill -P $$BACKEND_PID 2>/dev/null

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
	rm -rf $(BUILD_DIR)
	cd $(EMBED_DIR) && find . -not -name '.gitkeep' -not -name '.' -delete 2>/dev/null || true
