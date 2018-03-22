package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/rubenv/sql-migrate"
)

type StatusCommand struct {
}

func (c *StatusCommand) Help() string {
	helpText := `
Usage: sql-migrate status [options] ...

  Show migration status.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.

`
	return strings.TrimSpace(helpText)
}

func (c *StatusCommand) Synopsis() string {
	return "Show migration status"
}

func (c *StatusCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("status", flag.ContinueOnError)
	cmdFlags.Usage = func() { ui.Output(c.Help()) }
	var diffByMigration []*migrate.MigrationRecord

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

	rows := make(map[string]*statusRow)

	for _, m := range migrations {
		rows[m.Id] = &statusRow{
			ID:       m.Id,
			Migrated: false,
		}
	}

	for _, r := range records {
		if _, ok := rows[r.Id]; ok {
			rows[r.Id].Migrated = true
			rows[r.Id].AppliedAt = r.AppliedAt
		} else {
			row := &migrate.MigrationRecord{Id: r.Id, AppliedAt: r.AppliedAt}
			diffByMigration = append(diffByMigration, row)

		}
	}
	// fmt.Println(&diffByMigration)
	// log.warning("aa")
	for _, m := range migrations {
		if rows[m.Id].Migrated {
			table.Append([]string{
				m.Id,
				rows[m.Id].AppliedAt.String(),
			})
		} else {
			table.Append([]string{
				m.Id,
				"no",
			})
		}
	}

	table.Render()

	if &diffByMigration != nil {
		fmt.Println("\n=== Rollback Schema Diff ===")
		for _, k := range diffByMigration {
			fmt.Println(k.Id, k.AppliedAt)
		}
		fmt.Println("=== Rollback Schema Diff End ===")
	}

	return 0
}

type statusRow struct {
	ID        string
	Migrated  bool
	AppliedAt time.Time
}
