package main

import (
	"flag"
	"strings"

	migrate "github.com/rubenv/sql-migrate"
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
  -version               Run migrate up to a specific version, eg: 20221102092308-create-users or 20221102092308.
  -dryrun                Don't apply migrations, just print them.

`
	return strings.TrimSpace(helpText)
}

func (c *DownCommand) Synopsis() string {
	return "Undo a database migration"
}

func (c *DownCommand) Run(args []string) int {
	var limit int
	var version string
	var dryrun bool

	cmdFlags := flag.NewFlagSet("down", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 1, "Max number of migrations to apply.")
	cmdFlags.StringVar(&version, "version", "", "Migrate up to a specific version.")
	cmdFlags.BoolVar(&dryrun, "dryrun", false, "Don't apply migrations, just print them.")
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	err := ApplyMigrations(migrate.Down, dryrun, limit, version)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}
