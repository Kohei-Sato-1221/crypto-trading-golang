# help で表示するためコマンドの定義は以下のように記述
# {コマンド}: ## {コマンドの説明} ## {引数使用の場合のコマンドを記述}
help: ## print this message
	@echo ""
	@echo "Command list:"
	@printf "\033[36m%-35s\033[0m %s\n" "[Sub command]" "[Description]"
	@grep -E '^[/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | perl -pe 's%^([/a-zA-Z_-]+):.*?(##)%$$1 $$2%' | awk -F " *?## *?" '{printf "\033[36m%-35s\033[0m %s\n", $$3 ? $$3 : $$1, $$2}'
	@echo ''
	@echo '※ choose environment from [dev, stg, prod]'


TF_DIR=terraform
PROFILE_PREF=crypto-trading-20251113
TF_VERSION=1.13.5


run: ## run bitflyer trading ## run
	cd go && go run cmds/bifflyer_trading/main.go

run-binary: ## run bitflyer traging app via binary ## run-binary
	cd go && ./bfTradingApp

run-save-price-history: ## run savePriceHistoryJob ## run-save-price-history
	cd go && go run cmds/save_price_history_job/main.go

run-send-results: ## run sendResultsJob ## run-send-results
	cd go && go run cmds/send_results_job/main.go

build: ## build bitflyer trading app ## build
	rm -rf go/bfTradingApp
	cd go && go build cmds/bifflyer_trading/main.go && mv main bfTradingApp && chmod 500 bfTradingApp

db-up: ## start local PostgreSQL via docker-compose ## db-up
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do sleep 1; done
	@echo "PostgreSQL is ready on port 5433."

db-down: ## stop local PostgreSQL ## db-down
	docker compose down

test: ## run integration tests with docker-compose PostgreSQL ## test
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; do sleep 1; done
	@echo "PostgreSQL is ready."
	cd go && go test ./tests/ -v -count=1
	docker compose down

test-keep-db: ## run tests without stopping PostgreSQL (for repeated runs) ## test-keep-db
	cd go && go test ./tests/ -v -count=1

migrate-dump: ## Dump MySQL data from AWS RDS and convert to PostgreSQL format ## migrate-dump
	bash scripts/migration/dump_mysql.sh

migrate-import: ## Import dumped data to Supabase PostgreSQL ## migrate-import
	bash scripts/migration/import_to_supabase.sh

migrate-reset: ## Drop all tables in Supabase (for re-import) ## migrate-reset
	bash scripts/migration/reset_supabase.sh

tfenv: ## change terraform version ## tfenv
	tfenv use ${TF_VERSION}

fmt: ## format terraform code ## fmt
	terraform fmt -recursive

init: fmt tfenv ## terraform init ## init
	cd $(TF_DIR) && AWS_PROFILE=$(PROFILE_PREF) terraform init

plan: fmt tfenv ## terraform plan ## plan
	cd $(TF_DIR) && AWS_PROFILE=$(PROFILE_PREF) terraform plan

apply: fmt tfenv ## terraform apply ## apply
	cd $(TF_DIR) && AWS_PROFILE=$(PROFILE_PREF) terraform apply

destroy: fmt tfenv ## terraform destroy ## destroy
	cd $(TF_DIR) && AWS_PROFILE=$(PROFILE_PREF) terraform destroy
