package main

import (
	"flag"
	"strings"

	migrate "github.com/rubenv/sql-migrate"
)

// UpCommand is the method receiver
type UpCommand struct {
}

/*
Help shows the help text.
*/
func (c *UpCommand) Help() string {
	helpText := `
Usage: sql-migrate up [options] ...

  Migrates the database to the most recent version available.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=0               Limit the number of migrations (0 = unlimited).
  -dryrun                Don't apply migrations, just print them.

`
	return strings.TrimSpace(helpText)
}

/*
Synopsis returns the short description.
*/
func (c *UpCommand) Synopsis() string {
	return "Migrates the database to the most recent version available"
}

/*
Run executes via commandline parameters.
*/
func (c *UpCommand) Run(args []string) int {
	var limit int
	var dryrun bool

	cmdFlags := flag.NewFlagSet("up", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 0, "Max number of migrations to apply.")
	cmdFlags.BoolVar(&dryrun, "dryrun", false, "Don't apply migrations, just print them.")
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	err := ApplyMigrations(migrate.Up, dryrun, limit)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}
