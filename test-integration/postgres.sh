#!/bin/bash

# Tweak PATH for Travis
export PATH=$PATH:$HOME/gopath/bin

set -ex

# PostgreSQL
PG_OPTIONS="-config=test-integration/dbconfig.yml -env postgres"
sql-migrate status $PG_OPTIONS
sql-migrate up $PG_OPTIONS
sql-migrate down $PG_OPTIONS
sql-migrate redo $PG_OPTIONS
sql-migrate status $PG_OPTIONS

# MySQL
M_OPTIONS="-config=test-integration/dbconfig.yml -env mysql"
sql-migrate status $M_OPTIONS
sql-migrate up $M_OPTIONS
sql-migrate down $M_OPTIONS
sql-migrate redo $M_OPTIONS
sql-migrate status $M_OPTIONS
