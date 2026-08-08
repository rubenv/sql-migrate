package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"

	migrate "github.com/rubenv/sql-migrate"
)

type StatusCommand struct{}

type statusRow struct {
	Id        string
	Migrated  bool
	AppliedAt time.Time
}

func (*StatusCommand) Help() string {
	helpText := `
Usage: sql-migrate status [options] ...

  Show migration status.

Options:

  -config=dbconfig.yml   Configuration file to use.
  -env="development"     Environment.
  -output="json"         Print output in JSON format (default is table format).

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

	statusRows := buildStatusRows(migrations, records)

	if OutputFormat == "json" {
		if err := printStatusJSON(statusRows); err != nil {
			ui.Error(fmt.Sprintf("Could not encode JSON: %s", err))
			return 1
		}
	} else {
		printStatusTable(statusRows)
	}

	return 0
}

func buildStatusRows(migrations []*migrate.Migration, records []*migrate.MigrationRecord) []*statusRow {
	rows := make(map[string]*statusRow)

	for _, m := range migrations {
		rows[m.Id] = &statusRow{
			Id:       m.Id,
			Migrated: false,
		}
	}

	for _, r := range records {
		if row, ok := rows[r.Id]; ok {
			row.Migrated = true
			row.AppliedAt = r.AppliedAt
		} else {
			ui.Warn(fmt.Sprintf("Could not find migration file: %v", r.Id))
		}
	}

	var result []*statusRow
	for _, m := range migrations {
		result = append(result, rows[m.Id])
	}
	return result
}

func printStatusJSON(rows []*statusRow) error {
	type jsonRow struct {
		Migration string `json:"migration"`
		Applied   string `json:"applied"`
	}

	var output []jsonRow
	for _, r := range rows {
		applied := "no"
		if r.Migrated {
			applied = r.AppliedAt.Format(time.RFC3339)
		}
		output = append(output, jsonRow{
			Migration: r.Id,
			Applied:   applied,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func printStatusTable(rows []*statusRow) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Migration", "Applied"})
	table.SetColWidth(60)

	for _, r := range rows {
		applied := "no"
		if r.Migrated {
			applied = r.AppliedAt.Format("2006-01-02 15:04:05")
		}
		table.Append([]string{r.Id, applied})
	}

	table.Render()
}
