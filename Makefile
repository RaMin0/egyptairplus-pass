run:
	@set -o allexport && source .env && go run .
.PHONY: run

ci:
	@go run .
.PHONY: ci
