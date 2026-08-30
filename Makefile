# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: no-dirty
no-dirty:
	@test -z "$(shell git status --porcelain)"

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## audit: run quality control checks
.PHONY: audit
audit: test
	go mod tidy -diff
	go mod verify
	test -z "$(shell gofmt -l .)"
	GOOS=linux go vet ./...
	GOOS=windows go vet ./...
	GOOS=darwin go vet ./...
	GOOS=linux go build ./...
	GOOS=windows go build ./...

## test: run all tests
.PHONY: test
test:
	go test -race -buildvcs ./...

## tidy: tidy modfiles and format source
.PHONY: tidy
tidy:
	go mod tidy -v
	gofmt -w .
