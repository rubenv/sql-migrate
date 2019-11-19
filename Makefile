.DEFAULT_GOAL := help
.PHONY: start
MAKEFLAGS += --silent
PROJECTNAME=$(shell basename "$(PWD)")
TESTTIMEOUT=-timeout 30s

%::
	make
	@echo "\033[01;31m > type one of the targets above\033[00m"
	@echo

## tidy: runs go mod tidy
tidy:
	@tput reset
	@echo "\033[01;34m > Updating mod-file...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go mod tidy
	@echo "\033[01;32m > ready...\033[00m"

## check: runs linter golangci-lint
check:
	@tput reset
	@echo "\033[01;34m > Checking with golangci-lint...\033[00m"
	@golangci-lint run
	@echo "\033[01;32m > ready...\033[00m"

## check2: runs linter staticcheck
check2:
	@tput reset
	@echo "\033[01;34m > Checking with staticcheck...\033[00m"
	@staticcheck -checks all ./...
	@echo "\033[01;32m > ready...\033[00m"

## check3: runs linter revive
check3:
	@tput reset
	@echo "\033[01;34m > Checking with revive...\033[00m"
	@revive -formatter friendly ./...
	@echo "\033[01;32m > ready...\033[00m"

## test: test the applicatie (short)
test-s:
	@tput reset
	@echo "\033[01;34m > Testing (short)...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -count=1 $(TESTTIMEOUT) ./... -short
	@echo "\033[01;32m > ready...\033[00m"

## test: test the applicatie (full)
test:
	@tput reset
	@echo "\033[01;34m > Testing ...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -count=1 $(TESTTIMEOUT) ./...
	@echo "\033[01;32m > ready...\033[00m"


## test-c: test the applicatie with codecoverage
test-c:
	@make migrate
	@tput reset
	@echo "\033[01;34m > Testing with code codecoverage...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test $(TESTTIMEOUT) -coverprofile coverage.out ./...
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go tool cover -html coverage.out
	@echo "\033[01;32m > ready...\033[00m"

## test-v: test the applicatie verbose
test-v:
	@make migrate
	@tput reset
	@echo "\033[01;34m > Testing verbose...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go test -v -count=1 $(TESTTIMEOUT) ./...
	@echo "\033[01;32m > ready...\033[00m"


## format: runs gofmt
format:
	@tput reset
	@echo "\033[01;34m > format...\033[00m"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gofmt -s -w .
	@echo "\033[01;32m > ready...\033[00m"

makefile: help
help: Makefile

	@tput reset
	@echo "\033[01;34m > Choose a make command in "$(PROJECTNAME)":\033[00m"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo