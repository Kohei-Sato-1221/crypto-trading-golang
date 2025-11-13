FUNC_NAME=aws-scheduler
AWS_PROFILE=crypto-trading-20251113

help: ## print this message  ## help
	@echo ""
	@echo "Command list:"
	@printf "\033[36m%-35s\033[0m %s\n" "[Sub command]" "[Description]"
	@grep -E '^[/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | perl -pe 's%^([/a-zA-Z_-]+):.*?(##)%$$1 $$2%' | awk -F " *?## *?" '{printf "\033[36m%-35s\033[0m %s\n", $$3 ? $$3 : $$1, $$2}'
	@echo ""

build:## build lambda function ## build
	$(eval GOARCH := arm64)
	GOOS=linux GOARCH=${GOARCH} go build -o bootstrap main.go
	zip function.zip bootstrap

test:
	AWS_PROFILE=$(AWS_PROFILE) go test -v ./...

deploy: build ## deploy lambda function ## deploy
	AWS_PROFILE=$(AWS_PROFILE) aws lambda update-function-code --function-name $(FUNC_NAME) --zip-file fileb://function.zip

run: ## run lambda function ## run
	ENVIRONMENT=local go run main.go
