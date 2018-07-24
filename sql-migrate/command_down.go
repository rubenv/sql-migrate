package main

import (
	"flag"
	"strings"

	"github.com/17media/sql-migrate"
	. "github.com/17media/sql-migrate/sql-config"
)

type DownCommand struct {
}

func (c *DownCommand) Help() string {
	helpText := `
Usage: sql-migrate down [options] ...

  Undo a database migration.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=1               Limit the number of migrations (0 = unlimited).
  -dryrun                Don't apply migrations, just print them.
  -pt                    Using pt-online-schema-change to migration.
`
	return strings.TrimSpace(helpText)
}

func (c *DownCommand) Synopsis() string {
	return "Undo a database migration"
}

func (c *DownCommand) Run(args []string) int {
	var limit int
	var dryrun bool
	var pt bool

	cmdFlags := flag.NewFlagSet("down", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 1, "Max number of migrations to apply.")
	cmdFlags.BoolVar(&dryrun, "dryrun", false, "Don't apply migrations, just print them.")
	cmdFlags.BoolVar(&pt, "pt", false, "Using pt-online-schema-change to migration.")
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}
	// If pt is ture checking pt-online-schema-change whether exists
	if pt == true {
		err := CheckPTExist()
		if err != nil {
			ui.Error(err.Error())
			return 1
		}
	}

	err := ApplyMigrations(migrate.Down, dryrun, limit, pt)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}
