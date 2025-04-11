include .envrc # Make sure this line exists

.PHONY: run/tests
run/tests: vet
	go test -v ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet: fmt
	go vet ./...

# Use the correct DSN variable
.PHONY: run
run: vet
	go run ./cmd/web -addr=":4000" -dsn=${MOODNOTES_DB_DSN} # Changed here

# Use the correct DSN variable (Optional, for direct psql access)
.PHONY: db/psql
db/psql:
	psql ${MOODNOTES_DB_DSN} # Changed here

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all up database migrations
# Use the correct DSN variable
.PHONY: db/migrations/up
db/migrations/up:
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${MOODNOTES_DB_DSN} up # Changed here

## db/migrations/down: apply all down database migrations (Optional)
.PHONY: db/migrations/down
db/migrations/down:
	@echo 'Running down migrations...'
	migrate -path ./migrations -database ${MOODNOTES_DB_DSN} down # Changed here
