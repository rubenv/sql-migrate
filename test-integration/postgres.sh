#!/bin/bash

set -ex

export PATH=$PATH:`go env GOPATH`/bin

# TODO: Command-line tool tests here
sql-migrate --help
