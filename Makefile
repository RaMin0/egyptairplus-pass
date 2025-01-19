APP ?= egyptair

dev:
	@set -o allexport && source .env && go run ./cmd/${APP}
.PHONY: dev

run:
	@go run ./cmd/${APP}
.PHONY: run
