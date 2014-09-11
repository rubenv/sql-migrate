// +build go1.3

package command

import (
	"github.com/coopernurse/gorp"
	_ "github.com/denisenkom/go-mssqldb"
)

func init() {
	dialects["mssql"] = gorp.SqlServerDialect{}
}
