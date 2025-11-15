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

build: ## build bitflyer trading app ## build
	rm -rf go/bfTradingApp
	cd go && go build cmds/bifflyer_trading/main.go && mv main bfTradingApp && chmod 500 bfTradingApp

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
