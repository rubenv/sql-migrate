//go:build go1.3
// +build go1.3

package main

import (
	_ "github.com/denisenkom/go-mssqldb"
	"github.com/go-gorp/gorp/v3"
)

func init() {
	dialects["mssql"] = gorp.SqlServerDialect{}
}
