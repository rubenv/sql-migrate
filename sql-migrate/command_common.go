package main

import (
	"fmt"
	"strings"

	migrate "github.com/rubenv/sql-migrate"
)

func ApplyMigrations(dir migrate.MigrationDirection, dryrun bool, limit int, version string) error {
	env, err := GetEnvironment()
	if err != nil {
		return fmt.Errorf("Could not parse config: %s", err)
	}

	db, dialect, err := GetConnection(env)
	if err != nil {
		return err
	}
	defer db.Close()

	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}

	version = strings.TrimSpace(version)
	var targetVersionId int64 = 0
	if len(version) > 0 {
		tempMigration := migrate.Migration{Id: version}
		targetVersionId = tempMigration.VersionInt()
	}

	if dryrun {
		migrations, _, err := migrate.PlanMigration(db, dialect, source, dir, limit, targetVersionId)
		if err != nil {
			return fmt.Errorf("Cannot plan migration: %s", err)
		}

		for _, m := range migrations {
			PrintMigration(m, dir)
		}
	} else {
		n, err := migrate.ExecMax(db, dialect, source, dir, limit, targetVersionId)
		if err != nil {
			return fmt.Errorf("Migration failed: %s", err)
		}

		if n == 1 {
			ui.Output("Applied 1 migration")
		} else {
			ui.Output(fmt.Sprintf("Applied %d migrations", n))
		}
	}

	return nil
}

func PrintMigration(m *migrate.PlannedMigration, dir migrate.MigrationDirection) {
	if dir == migrate.Up {
		ui.Output(fmt.Sprintf("==> Would apply migration %s (up)", m.Id))
		for _, q := range m.Up {
			ui.Output(q)
		}
	} else if dir == migrate.Down {
		ui.Output(fmt.Sprintf("==> Would apply migration %s (down)", m.Id))
		for _, q := range m.Down {
			ui.Output(q)
		}
	} else {
		panic("Not reached")
	}
}
