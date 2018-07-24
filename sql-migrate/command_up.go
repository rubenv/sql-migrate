package main

import (
	"flag"
	"strings"

	"github.com/17media/sql-migrate"
	. "github.com/17media/sql-migrate/sql-config"
)

type UpCommand struct {
}

func (c *UpCommand) Help() string {
	helpText := `
Usage: sql-migrate up [options] ...

  Migrates the database to the most recent version available.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=0               Limit the number of migrations (0 = unlimited).
  -dryrun                Don't apply migrations, just print them.
  -pt                    Using pt-online-schema-change to migration.

`
	return strings.TrimSpace(helpText)
}

func (c *UpCommand) Synopsis() string {
	return "Migrates the database to the most recent version available"
}

func (c *UpCommand) Run(args []string) int {
	var limit int
	var dryrun bool
	var pt bool

	cmdFlags := flag.NewFlagSet("up", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 0, "Max number of migrations to apply.")
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

	err := ApplyMigrations(migrate.Up, dryrun, limit, pt)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}
