package main

import (
	"flag"
	"strings"

	"github.com/posener/complete"

	migrate "github.com/rubenv/sql-migrate"
)

type UpCommand struct{}

func (*UpCommand) Help() string {
	helpText := `
Usage: sql-migrate up [options] ...

  Migrates the database to the most recent version available.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=0               Limit the number of migrations (0 = unlimited).
  -version               Run migrate up to a specific version, eg: the version number of migration 1_initial.sql is 1.
  -dryrun                Don't apply migrations, just print them.

`
	return strings.TrimSpace(helpText)
}

func (*UpCommand) Synopsis() string {
	return "Migrates the database to the most recent version available"
}

func (*UpCommand) AutocompleteArgs() complete.Predictor {
	return nil
}

func (*UpCommand) AutocompleteFlags() complete.Flags {
	f := complete.Flags{
		"-dryrun":  complete.PredictNothing,
		"-limit":   complete.PredictAnything,
		"-version": complete.PredictAnything,
	}
	ConfigFlagsCompletions(f)
	return f
}

func (c *UpCommand) Run(args []string) int {
	var limit int
	var version int64
	var dryrun bool

	cmdFlags := flag.NewFlagSet("up", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 0, "Max number of migrations to apply.")
	cmdFlags.Int64Var(&version, "version", -1, "Migrate up to a specific version.")
	cmdFlags.BoolVar(&dryrun, "dryrun", false, "Don't apply migrations, just print them.")
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	err := ApplyMigrations(migrate.Up, dryrun, limit, version)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}
