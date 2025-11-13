include ./envs/.db.env

# URL encode password (replace & with %26)
ENCODED_PASSWORD = $(subst &,%26,$(DB_PASSWORD))

# Database URL for Atlas (standard MySQL connection string format)
DB_URL = mysql://$(DB_USER):$(ENCODED_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?parseTime=true

# help で表示するためコマンドの定義は以下のように記述
# {コマンド}: ## {コマンドの説明} ## {引数使用の場合のコマンドを記述}
help: ## print this message ## help
	@echo ""
	@echo "Command list:"
	@printf "\033[36m%-35s\033[0m %s\n" "[Sub command]" "[Description]"
	@grep -E '^[/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | perl -pe 's%^([/a-zA-Z_-]+):.*?(##)%$$1 $$2%' | awk -F " *?## *?" '{printf "\033[36m%-35s\033[0m %s\n", $$3 ? $$3 : $$1, $$2}'
	@echo ""



install-migration-tool: ## install atlas(for mac/linux) ## install-migration-tool
	@which atlas > /dev/null || curl -sSf https://atlasgo.sh | sh

uninstall-migration-tool: ## uninstall atlas(for mac/linux) ## uninstall-migration-tool
	sudo rm -f $(shell which atlas)

install-migration-tool-win: ## install atlas(for windows) ## install-migration-tool-win
	@echo "not implemented"

atlas-status: ## check migration status ## atlas-status
	atlas migrate status \
		--config "file://crypto-trading-db/atlas/atlas.hcl" \
		--dir "file://crypto-trading-db/atlas/migrations" \
		-u "$(DB_URL)"

atlas-apply: ## apply migration file ## atlas-apply
	atlas migrate apply $(if $(allowdirty),--allow-dirty) \
		--config "file://crypto-trading-db/atlas/atlas.hcl" \
		--dir "file://crypto-trading-db/atlas/migrations" \
		-u "$(DB_URL)"

atlas-diff: ## diff schema ## atlas-diff n={migration_file_name}
ifeq "$(n)" ""
	@echo "migration file name is required"
	exit 1
endif
	atlas migrate diff "${n}" \
		--config "file://crypto-trading-db/atlas/atlas.hcl" \
		--dir "file://crypto-trading-db/atlas/migrations" \
		--to "file://crypto-trading-db/atlas/schema.hcl" \
		--dev-url "docker://mysql/8/dev" \
		--format '{{ sql . "  " }}'
	make atlas-hash

atlas-hash: ## update hash ## atlas-hash
	atlas migrate hash \
		--config "file://crypto-trading-db/atlas/atlas.hcl" \
		--dir "file://crypto-trading-db/atlas/migrations"

atlas-clean: ## clean db migration file ## atlas-clean
	atlas schema clean --auto-approve \
		--config "file://crypto-trading-db/atlas/atlas.hcl" \
		-u "$(DB_URL)"

reset-db: ## reset db ## reset-db
	make atlas-clean
	make atlas-hash
	make atlas-apply

up: install-migration-tool ## prepare db by atlas ## up
	make atlas-apply
	
