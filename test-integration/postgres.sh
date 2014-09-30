#!/bin/bash

# Tweak PATH for Travis
export PATH=$PATH:$HOME/gopath/bin

set -ex

PG_OPTIONS="-config=test-integration/dbconfig.yml -env postgres"
sql-migrate status $PG_OPTIONS
sql-migrate up $PG_OPTIONS
sql-migrate down $PG_OPTIONS
sql-migrate redo $PG_OPTIONS
sql-migrate status $PG_OPTIONS
