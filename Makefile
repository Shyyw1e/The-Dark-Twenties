ifneq (,$(wildcard .env))
include .env
export
endif

ifeq ($(OS),Windows_NT)
	SHELL := powershell.exe
	.SHELLFLAGS := -NoProfile -ExecutionPolicy Bypass -Command
	DETECTED_OS := windows
	NULL_DEVICE := NUL
else
	SHELL := /bin/sh
	.SHELLFLAGS := -c
	DETECTED_OS := linux
	NULL_DEVICE := /dev/null
endif

GOOSE ?= goose
GOOSE_VERSION ?= v3.26.0

USER_MIGRATIONS := services/user-service/migrations
SUBSCRIPTION_MIGRATIONS := services/subscription-service/migrations
BILLING_MIGRATIONS := services/billing-service/migrations
TUNNEL_MIGRATIONS := services/tunnel-service/migrations
CONFIG_MIGRATIONS := services/config-service/migrations

.PHONY: help os test goose-install run-user-service \
	migrate-status migrate-up migrate-down \
	migrate-user-status migrate-user-up migrate-user-down migrate-user-reset \
	migrate-subscription-status migrate-subscription-up migrate-subscription-down migrate-subscription-reset \
	migrate-billing-status migrate-billing-up migrate-billing-down migrate-billing-reset \
	migrate-tunnel-status migrate-tunnel-up migrate-tunnel-down migrate-tunnel-reset \
	migrate-config-status migrate-config-up migrate-config-down migrate-config-reset

help:
	@echo "The Dark Twenties dev commands"
	@echo ""
	@echo "Detected OS: $(DETECTED_OS)"
	@echo ""
	@echo "Common:"
	@echo "  make test"
	@echo "  make goose-install"
	@echo "  make run-user-service"
	@echo ""
	@echo "All migrations:"
	@echo "  make migrate-status"
	@echo "  make migrate-up"
	@echo "  make migrate-down"
	@echo ""
	@echo "Per service:"
	@echo "  make migrate-user-up"
	@echo "  make migrate-subscription-up"
	@echo "  make migrate-billing-up"
	@echo "  make migrate-tunnel-up"
	@echo "  make migrate-config-up"

os:
	@echo "$(DETECTED_OS)"

test:
	go test ./...

goose-install:
	go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

run-user-service:
	go run ./services/user-service/cmd/user-service

migrate-status: migrate-user-status migrate-subscription-status migrate-billing-status migrate-tunnel-status migrate-config-status

migrate-up: migrate-user-up migrate-subscription-up migrate-billing-up migrate-tunnel-up migrate-config-up

migrate-down: migrate-config-down migrate-tunnel-down migrate-billing-down migrate-subscription-down migrate-user-down

migrate-user-status:
	$(GOOSE) -dir "$(USER_MIGRATIONS)" postgres "$(USER_SERVICE_POSTGRES_DSN)" status

migrate-user-up:
	$(GOOSE) -dir "$(USER_MIGRATIONS)" postgres "$(USER_SERVICE_POSTGRES_DSN)" up

migrate-user-down:
	$(GOOSE) -dir "$(USER_MIGRATIONS)" postgres "$(USER_SERVICE_POSTGRES_DSN)" down

migrate-user-reset:
	$(GOOSE) -dir "$(USER_MIGRATIONS)" postgres "$(USER_SERVICE_POSTGRES_DSN)" reset

migrate-subscription-status:
	$(GOOSE) -dir "$(SUBSCRIPTION_MIGRATIONS)" postgres "$(SUBSCRIPTION_SERVICE_POSTGRES_DSN)" status

migrate-subscription-up:
	$(GOOSE) -dir "$(SUBSCRIPTION_MIGRATIONS)" postgres "$(SUBSCRIPTION_SERVICE_POSTGRES_DSN)" up

migrate-subscription-down:
	$(GOOSE) -dir "$(SUBSCRIPTION_MIGRATIONS)" postgres "$(SUBSCRIPTION_SERVICE_POSTGRES_DSN)" down

migrate-subscription-reset:
	$(GOOSE) -dir "$(SUBSCRIPTION_MIGRATIONS)" postgres "$(SUBSCRIPTION_SERVICE_POSTGRES_DSN)" reset

migrate-billing-status:
	$(GOOSE) -dir "$(BILLING_MIGRATIONS)" postgres "$(BILLING_SERVICE_POSTGRES_DSN)" status

migrate-billing-up:
	$(GOOSE) -dir "$(BILLING_MIGRATIONS)" postgres "$(BILLING_SERVICE_POSTGRES_DSN)" up

migrate-billing-down:
	$(GOOSE) -dir "$(BILLING_MIGRATIONS)" postgres "$(BILLING_SERVICE_POSTGRES_DSN)" down

migrate-billing-reset:
	$(GOOSE) -dir "$(BILLING_MIGRATIONS)" postgres "$(BILLING_SERVICE_POSTGRES_DSN)" reset

migrate-tunnel-status:
	$(GOOSE) -dir "$(TUNNEL_MIGRATIONS)" postgres "$(TUNNEL_SERVICE_POSTGRES_DSN)" status

migrate-tunnel-up:
	$(GOOSE) -dir "$(TUNNEL_MIGRATIONS)" postgres "$(TUNNEL_SERVICE_POSTGRES_DSN)" up

migrate-tunnel-down:
	$(GOOSE) -dir "$(TUNNEL_MIGRATIONS)" postgres "$(TUNNEL_SERVICE_POSTGRES_DSN)" down

migrate-tunnel-reset:
	$(GOOSE) -dir "$(TUNNEL_MIGRATIONS)" postgres "$(TUNNEL_SERVICE_POSTGRES_DSN)" reset

migrate-config-status:
	$(GOOSE) -dir "$(CONFIG_MIGRATIONS)" postgres "$(CONFIG_SERVICE_POSTGRES_DSN)" status

migrate-config-up:
	$(GOOSE) -dir "$(CONFIG_MIGRATIONS)" postgres "$(CONFIG_SERVICE_POSTGRES_DSN)" up

migrate-config-down:
	$(GOOSE) -dir "$(CONFIG_MIGRATIONS)" postgres "$(CONFIG_SERVICE_POSTGRES_DSN)" down

migrate-config-reset:
	$(GOOSE) -dir "$(CONFIG_MIGRATIONS)" postgres "$(CONFIG_SERVICE_POSTGRES_DSN)" reset
