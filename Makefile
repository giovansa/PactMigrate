# Optional helpers for the pactmigrate library (not required to use the package).
# Make uses name=value (not --flags). Example: make migration name=add_users_table

.PHONY: help build build-server run-server run-dashboard test config config-mysql config-server migration migration-rehash postgresql

# JSONC source and JSON output (defaults overwrite apps/pactmigrate-cli/config.json).
JSONC?=apps/pactmigrate-cli/config.example.jsonc
OUT?=apps/pactmigrate-cli/config.json

help:
	@echo "Targets:"
	@echo "  make build                              # go build -o migrate ./apps/pactmigrate-cli"
	@echo "  make build-server                       # go build -o server ./apps/server"
	@echo "  make run-server                         # go run ./apps/server -config $(SERVER_CONFIG)"
	@echo "  make run-dashboard                      # npm run dev (apps/dashboard)"
	@echo "  make test                               # go test all Go workspace modules"
	@echo "  make config                             # JSONC -> JSON (defaults: $(JSONC) -> $(OUT))"
	@echo "  make config-mysql                       # apps/pactmigrate-cli/config.mysql.example.jsonc -> $(OUT)"
	@echo "  make config-server                      # apps/server/config.example.jsonc -> apps/server/config.json"
	@echo "  make migration name=your_migration_name"
	@echo "      Create apps/pactmigrate-cli/migrations/<ts>_<sha8>_<title>.sql"
	@echo "  make migration-rehash                     # rehash all apps/pactmigrate-cli/migrations/*.sql"
	@echo "  make migration-rehash FILE=path/to/one.sql   # rehash a single file"
	@echo ""
	@echo "Example: make migration name=add_users_table"

build:
	go build -o migrate ./apps/pactmigrate-cli

build-server:
	go build -o server ./apps/server

SERVER_CONFIG?=apps/server/config.json

run-server:
	go run ./apps/server -config "$(SERVER_CONFIG)"

run-dashboard:
	npm --prefix apps/dashboard run dev

test:
	go test ./apps/pactmigrate-cli/... ./apps/server/... ./packages/core/...

config:
	go run ./apps/pactmigrate-cli/tools/jsonc2json $(JSONC) $(OUT)

config-mysql:
	$(MAKE) config JSONC=apps/pactmigrate-cli/config.mysql.example.jsonc

config-server:
	$(MAKE) config JSONC=apps/server/config.example.jsonc OUT=apps/server/config.json

migration:
	@scripts/new-migration.sh "$(or $(name),$(NAME))"

migration-rehash:
	@if [ -n "$(FILE)" ]; then scripts/rehash-migration.sh "$(FILE)"; else scripts/rehash-migration.sh; fi

postgresql:
	docker run --name local-postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres -d postgres:latest
