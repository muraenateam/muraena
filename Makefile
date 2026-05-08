BUILD   ?= build
TARGET  ?= muraena
GO      ?= go

.PHONY: all setup up up-full down logs logs-all build test clean help \
        build_with_race_detector buildall fmt

all: build

## setup    — interactive plug-and-play setup (generates config, starts everything)
setup:
	@bash setup.sh

## up       — start Muraena + Redis in Docker
up:
	docker compose up -d

## up-full  — start Muraena + Redis + Necrobrowser-NG in Docker
up-full:
	docker compose --profile necrobrowser up -d

## down     — stop all Docker services
down:
	docker compose --profile necrobrowser down

## logs     — tail Muraena logs
logs:
	docker compose logs -f muraena

## logs-all — tail all service logs
logs-all:
	docker compose --profile necrobrowser logs -f

## build    — compile Muraena binary locally
build:
	$(GO) build -trimpath -ldflags="-s -w" -o $(BUILD)/$(TARGET) .

build_with_race_detector:
	$(GO) build -race -o $(BUILD)/$(TARGET) .

buildall:
	env GOOS=darwin  GOARCH=amd64 $(GO) build -o $(BUILD)/macos/$(TARGET) .
	env GOOS=linux   GOARCH=amd64 $(GO) build -o $(BUILD)/linux/$(TARGET) .
	env GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD)/windows/$(TARGET).exe .

## test     — run all tests
test:
	$(GO) test ./...

## fmt      — gofmt all source packages
fmt:
	gofmt -s -w core log session module

## clean    — remove build artifacts
clean:
	rm -rf $(BUILD)
	@echo "Run 'docker compose down -v' to also remove Redis data."

## help     — show this message
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
