include ./envs/.db.env
# Supabase(PostgreSQL) の接続情報。存在しなくてもエラーにしない（MySQL側の操作は継続可能）
-include ./envs/.pg.env

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

# === PostgreSQL commands ===

PG_LOCAL_URL = postgresql://postgres:postgres@localhost:5433/crypto_trading

pg-status: ## check PostgreSQL migration status ## pg-status
	atlas migrate status \
		--dir "file://crypto-trading-db-postgres/migrations" \
		-u "$(PG_LOCAL_URL)"

pg-apply: ## apply PostgreSQL migration ## pg-apply
	atlas migrate apply \
		--dir "file://crypto-trading-db-postgres/migrations" \
		-u "$(PG_LOCAL_URL)"

pg-init: ## initialize local PostgreSQL with init SQL ## pg-init
	psql "$(PG_LOCAL_URL)" -f crypto-trading-db-postgres/init/001_init.sql

# === Supabase (production PostgreSQL) commands ===
# 接続情報は envs/.pg.env の PG_SUPABASE_URL（gitignore 済み）。
# ★ 必ず 5432(session pooler) を使うこと。6543 は transaction pooling のため
#   Atlas のアドバイザリロック / セッション状態が壊れる。
# ★ 初回のみ baseline 指定が必要:
#     make pg-remote-apply baseline=20260506000000
PG_MIGRATIONS_DIR = crypto-trading-db-postgres/migrations

pg-remote-check: ## verify PG_SUPABASE_URL is set ## pg-remote-check
ifeq "$(PG_SUPABASE_URL)" ""
	@echo "PG_SUPABASE_URL is not set. Create db/envs/.pg.env first."
	@exit 1
endif
	@echo "PG_SUPABASE_URL is set."

pg-remote-status: pg-remote-check ## check Supabase migration status ## pg-remote-status
	@atlas migrate status \
		--dir "file://$(PG_MIGRATIONS_DIR)" \
		-u "$(PG_SUPABASE_URL)"

pg-remote-dry-run: pg-remote-check ## dry-run migration against Supabase ## pg-remote-dry-run baseline={version}
	@atlas migrate apply --dry-run $(if $(baseline),--baseline $(baseline)) \
		--dir "file://$(PG_MIGRATIONS_DIR)" \
		-u "$(PG_SUPABASE_URL)"

pg-remote-apply: pg-remote-check ## apply migration to Supabase ## pg-remote-apply baseline={version}
	@atlas migrate apply $(if $(baseline),--baseline $(baseline)) \
		--dir "file://$(PG_MIGRATIONS_DIR)" \
		-u "$(PG_SUPABASE_URL)"

