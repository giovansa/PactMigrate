module pactmigrate.local/apps/pactmigrate-cli

go 1.21

require (
	github.com/go-sql-driver/mysql v1.8.1
	github.com/jackc/pgx/v5 v5.7.2
	github.com/tidwall/jsonc v0.3.3
	pactmigrate.local/packages/core v0.0.0
)

replace pactmigrate.local/packages/core => ../../packages/core

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)
