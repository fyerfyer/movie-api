include .envrc

## help: print this help message 
.PHONY: help
help: 
	@echo 'Usage'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## run/api: run the cmd/api application
.PHONY: run
run:
	@go run ./cmd/api -db-dsn=${GREENLIGHT_DB_DSN}

## psql: connect to the database using psql
.PHONY: psql
psql:
	psql ${GREENLIGHT_DB_DSN_MIGRATE}

## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit:vendor
	@echo 'Formatting code...'
		go fmt ./...
	@echo 'Vetting code...'
		go vet ./...
		staticcheck ./...
	@echo 'Running tests...'
		go test -race -vet=off ./...

## vendor: tidy and vendor dependencies
.PHONY: vendor
vendor:
	@echo 'Tidying and verifying module dependencies...'
		go mod tidy
		go mod verify
	@echo 'Vendoring dependencies...'
		go mod vendor

## build/api: build the cmd/api application
.PHONY: build
build:
	@echo 'Building cmd/api...'
		GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api

## print-dsn: print the database DSN from .envrc
.PHONY: print-dsn
print-dsn:
	@echo $(GREENLIGHT_DB_DSN)
