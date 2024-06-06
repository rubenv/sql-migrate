package main

import (
	"flag"
	"strings"

	"github.com/posener/complete"

	migrate "github.com/rubenv/sql-migrate"
)

type DownCommand struct{}

func (*DownCommand) Help() string {
	helpText := `
Usage: sql-migrate down [options] ...

  Undo a database migration.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=1               Limit the number of migrations (0 = unlimited).
  -version               Run migrate down to a specific version, eg: the version number of migration 1_initial.sql is 1.
  -dryrun                Don't apply migrations, just print them.

`
	return strings.TrimSpace(helpText)
}

func (*DownCommand) Synopsis() string {
	return "Undo a database migration"
}

func (*DownCommand) AutocompleteArgs() complete.Predictor {
	return nil
}

func (*DownCommand) AutocompleteFlags() complete.Flags {
	f := complete.Flags{
		"-limit":   complete.PredictAnything,
		"-version": complete.PredictAnything,
		"-dryrun":  complete.PredictNothing,
	}
	ConfigFlagsCompletions(f)
	return f
}

func (c *DownCommand) Run(args []string) int {
	var limit int
	var version int64
	var dryrun bool

	cmdFlags := flag.NewFlagSet("down", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 1, "Max number of migrations to apply.")
	cmdFlags.Int64Var(&version, "version", -1, "Migrate down to a specific version.")
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
