package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	migrate "github.com/rubenv/sql-migrate"
)

type StatusCommand struct{}

func (*StatusCommand) Help() string {
	helpText := `
Usage: sql-migrate status [options] ...

  Show migration status.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.

`
	return strings.TrimSpace(helpText)
}

func (*StatusCommand) Synopsis() string {
	return "Show migration status"
}

func (c *StatusCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("status", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
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
	defer db.Close()

	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}
	migrations, err := source.FindMigrations()
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	records, err := migrate.GetMigrationRecords(db, dialect)
	if err != nil {
		ui.Error(err.Error())
		return 1
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Migration", "Applied"})
	table.SetColWidth(60)

	// Build a map of source migrations for quick lookup
	sourceMigrations := make(map[string]*migrate.Migration)
	for _, m := range migrations {
		sourceMigrations[m.Id] = m
	}

	// Build a map of applied migrations for quick lookup
	applied := make(map[string]*migrate.MigrationRecord)
	for _, m := range records {
		applied[m.Id] = m
	}

	// Print status for all migrations in the source
	for _, m := range migrations {
		if rec, ok := applied[m.Id]; ok {
			table.Append([]string{
				m.Id,
				rec.AppliedAt.Format(time.RFC3339),
			})
		} else {
			table.Append([]string{
				m.Id,
				"no",
			})
		}
	}

	// Print status for migrations in DB but not in source
	for _, m := range records {
		if _, ok := sourceMigrations[m.Id]; !ok {
			table.Append([]string{
				m.Id,
				fmt.Sprintf("yes (missing in source, applied at %s)", m.AppliedAt.Format(time.RFC3339)),
			})
		}
	}

	table.Render()

	return 0
}

type statusRow struct {
	Id        string
	Migrated  bool
	AppliedAt time.Time
}
