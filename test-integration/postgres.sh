#!/bin/bash

set -ex

# Tweak PATH for Travis
export PATH=$PATH:$HOME/gopath/bin

# TODO: Command-line tool tests here
sql-migrate --help
