#!/bin/bash

# Tweak PATH for Travis
export PATH=$PATH:$HOME/gopath/bin

set -ex

sql-migrate status -config=test-integration/dbconfig.yml -env postgres
