package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/posener/complete"

	migrate "github.com/rubenv/sql-migrate"
)

type SkipCommand struct{}

func (*SkipCommand) Help() string {
	helpText := `
Usage: sql-migrate skip [options] ...

  Set the database level to the most recent version available, without actually running the migrations.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -limit=0               Limit the number of migrations (0 = unlimited).

`
	return strings.TrimSpace(helpText)
}

func (*SkipCommand) Synopsis() string {
	return "Sets the database level to the most recent version available, without running the migrations"
}

func (*SkipCommand) AutocompleteArgs() complete.Predictor {
	return nil
}

func (*SkipCommand) AutocompleteFlags() complete.Flags {
	f := complete.Flags{
		"-limit": complete.PredictAnything,
	}
	ConfigFlagsCompletions(f)
	return f
}

func (c *SkipCommand) Run(args []string) int {
	var limit int

	cmdFlags := flag.NewFlagSet("up", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	cmdFlags.IntVar(&limit, "limit", 0, "Max number of migrations to skip.")
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	err := SkipMigrations(migrate.Up, limit)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	return 0
}

func SkipMigrations(dir migrate.MigrationDirection, limit int) error {
	env, err := GetEnvironment()
	if err != nil {
		return fmt.Errorf("Could not parse config: %w", err)
	}

	db, dialect, err := GetConnection(env)
	if err != nil {
		return err
	}
	defer db.Close()

	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}

	n, err := migrate.SkipMax(db, dialect, source, dir, limit)
	if err != nil {
		return fmt.Errorf("Migration failed: %w", err)
	}

	switch n {
	case 0:
		ui.Output("All migrations have already been applied")
	case 1:
		ui.Output("Skipped 1 migration")
	default:
		ui.Output(fmt.Sprintf("Skipped %d migrations", n))
	}

	return nil
}
