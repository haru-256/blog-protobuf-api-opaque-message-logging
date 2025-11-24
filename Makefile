.DEFAULT_GOAL := help

.PHONY: init
init: ## Initial setup
	mise install
	buf config init

.PHONY: generate
generate:  ## Generate gRPC code
	buf dep update
	buf generate

.PHONY: lint
lint:  ## Lint proto and go files
	buf lint
	golangci-lint run --config=./.golangci.yml ./...

.PHONY: fmt
fmt:  ## Format proto and go files
	buf format -w .
	go fmt ./...

.PHONY: run-server-normal
run-server-normal: ## API_OPAQUEの対応なしでサーバーを起動
	go run ./cmd/server

.PHONY: run-server-parsed
run-server-parsed: ## API_OPAQUEの対応ありでサーバーを起動
	go run ./cmd/server --parsed

.PHONY: list-server-services
list-server-services: ## List server services via reflection
	buf curl http://localhost:8081 --list-methods --http2-prior-knowledge

.PHONY: get-user
get-user: ## gRPCでGetUserを呼び出す
	buf curl --protocol grpc --http2-prior-knowledge \
		--schema . \
		--data '{"user_id": "1"}' \
		http://localhost:8081/myservice.v1.MyService/GetUser

.PHONY: help
help: ## Show options
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
