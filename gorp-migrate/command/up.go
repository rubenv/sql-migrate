package command

import (
	"database/sql"
	"flag"
	"fmt"
	"strings"

	"github.com/coopernurse/gorp"
	"github.com/kr/pretty"
	"github.com/mitchellh/cli"
	"github.com/rubenv/gorp-migrate"
)

type UpCommand struct {
	Ui cli.Ui
}

func (c *UpCommand) Help() string {
	helpText := `
Usage: gorp-migrate up [options] ...

  Migrates the database to the most recent version available.

Options:

  -config=config.yml   Configuration file to use.
  -env=""              Environment (defaults to first defined).
`
	return strings.TrimSpace(helpText)
}

func (c *UpCommand) Synopsis() string {
	return "Migrates the database to the most recent version available"
}

func (c *UpCommand) Run(args []string) int {
	cmdFlags := flag.NewFlagSet("up", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	ConfigFlags(cmdFlags)

	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	env, err := GetEnvironment()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Could not parse config: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("%# v", pretty.Formatter(env)))

	db, err := sql.Open(env.Dialect, env.DataSource)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Cannot connect to database: %s", err))
		return 1
	}

	dialect, exists := dialects[env.Dialect]
	if !exists {
		c.Ui.Error(fmt.Sprintf("Unsupported dialect: %s", env.Dialect))
		return 1
	}

	dbmap := &gorp.DbMap{Db: db, Dialect: dialect}
	source := migrate.FileMigrationSource{
		Dir: env.Dir,
	}

	n, err := migrate.Exec(dbmap, source, migrate.Up)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Migration failed: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Applied %d migrations", n))

	return 0
}
