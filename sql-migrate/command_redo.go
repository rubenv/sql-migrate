package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/17media/sql-migrate"
	. "github.com/17media/sql-migrate/sql-config"
)

type RedoCommand struct {
}

func (c *RedoCommand) Help() string {
	helpText := `
Usage: sql-migrate redo [options] ...

  Reapply the last migration.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -dryrun                Don't apply migrations, just print them.
  -pt                    Using pt-online-schema-change to migration.

`
	return strings.TrimSpace(helpText)
}

func (c *RedoCommand) Synopsis() string {
	return "Reapply the last migration"
}

func (c *RedoCommand) Run(args []string) int {
	var dryrun bool
	var pt bool

	cmdFlags := flag.NewFlagSet("redo", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
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

	env, err := GetEnvironment()
	if err != nil {
		ui.Error(fmt.Sprintf("Could not parse config: %s", err))
		return 1
	}

	db, dialect, err := GetConnection(env)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}

	migrations, _, err := migrate.PlanMigration(db, dialect, source, migrate.Down, 1)
	if len(migrations) == 0 {
		ui.Output("Nothing to do!")
		return 0
	}

	if dryrun {
		PrintMigration(migrations[0], migrate.Down)
		PrintMigration(migrations[0], migrate.Up)
	} else {
		_, err := migrate.ExecMax(db, dialect, source, migrate.Down, 1, pt)
		if err != nil {
			ui.Error(fmt.Sprintf("Migration (down) failed: %s", err))
			return 1
		}

		_, err = migrate.ExecMax(db, dialect, source, migrate.Up, 1, pt)
		if err != nil {
			ui.Error(fmt.Sprintf("Migration (up) failed: %s", err))
			return 1
		}

		ui.Output(fmt.Sprintf("Reapplied migration %s.", migrations[0].Id))
	}

	return 0
}
