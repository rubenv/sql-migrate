package main

import (
	"fmt"

	"github.com/rubenv/gorp-migrate"
)

func ApplyMigrations(dir migrate.MigrationDirection, dryrun bool, limit int) error {
	env, err := GetEnvironment()
	if err != nil {
		return fmt.Errorf("Could not parse config: %s", err)
	}

	dbmap, err := GetConnection(env)
	if err != nil {
		return err
	}

	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}

	if dryrun {
		migrations, _, err := migrate.PlanMigration(dbmap, source, dir, limit)
		if err != nil {
			return fmt.Errorf("Cannot plan migration: %s", err)
		}

		for _, m := range migrations {
			ui.Output(fmt.Sprintf("==> Would apply migration %s", m.Id))
			for _, q := range m.Queries {
				ui.Output(q)
			}
		}

	} else {
		n, err := migrate.ExecMax(dbmap, source, dir, limit)
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
