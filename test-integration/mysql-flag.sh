#!/bin/bash

# Tweak PATH for Travis
export PATH=$PATH:$HOME/gopath/bin

OPTIONS="-config=test-integration/dbconfig.yml -env mysql_noflag"

output=$(mktemp $TMPDIR/mysql-flag.XXXXXX)

set -ex

sql-migrate status $OPTIONS | tee $output
cat $output | grep -q "Make sure that the parseTime option is supplied"
