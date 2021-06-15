package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/rubenv/sql-migrate"
	"gopkg.in/gorp.v1"
	"gopkg.in/yaml.v2"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var dialects = map[string]gorp.Dialect{
	"sqlite3":  gorp.SqliteDialect{},
	"postgres": gorp.PostgresDialect{},
	"mysql":    gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"},
}

var ConfigFile string
var ConfigEnvironment string

func ConfigFlags(f *flag.FlagSet) {
	f.StringVar(&ConfigFile, "config", "dbconfig.yml", "Configuration file to use.")
	f.StringVar(&ConfigEnvironment, "env", "development", "Environment to use.")
}

type Environment struct {
	Dialect    string `yaml:"dialect" envconfig:"DIALECT"`
	DataSource string `yaml:"datasource" envconfig:"DATASOURCE"`
	Dir        string `yaml:"dir" envconfig:"DIR"`
	TableName  string `yaml:"table" envconfig:"TABLE"`
	SchemaName string `yaml:"schema" envconfig:"SCHEMA"`
}

func ConfigPresent() bool {
	_, err := os.Stat(ConfigFile)
	return !os.IsNotExist(err)
}

func ReadEnv() (*Environment, error) {
	env := &Environment{}
	if err := envconfig.Process("", env); err != nil {
		return nil, err
	}
	return env, nil
}

func ReadEnvFromFile() (*Environment, error) {
	file, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}

	config := make(map[string]*Environment)
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	env := config[ConfigEnvironment]
	if env == nil {
		return nil, errors.New("No environment: " + ConfigEnvironment)
	}

	return env, nil
}

func GetEnvironment() (*Environment, error) {
	var env *Environment
	if ConfigPresent() {
		var err error
		env, err = ReadEnvFromFile()
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		env, err = ReadEnv()
		if err != nil {
			return nil, err
		}
	}

	if env.Dialect == "" {
		return nil, errors.New("No dialect specified")
	}

	if env.DataSource == "" {
		return nil, errors.New("No data source specified")
	}
	env.DataSource = os.ExpandEnv(env.DataSource)

	if env.Dir == "" {
		env.Dir = "migrations"
	}

	if env.TableName != "" {
		migrate.SetTable(env.TableName)
	}

	if env.SchemaName != "" {
		migrate.SetSchema(env.SchemaName)
	}

	return env, nil
}

func GetConnection(env *Environment) (*sql.DB, string, error) {
	db, err := sql.Open(env.Dialect, env.DataSource)
	if err != nil {
		return nil, "", fmt.Errorf("Cannot connect to database: %s", err)
	}

	// Make sure we only accept dialects that were compiled in.
	_, exists := dialects[env.Dialect]
	if !exists {
		return nil, "", fmt.Errorf("Unsupported dialect: %s", env.Dialect)
	}

	return db, env.Dialect, nil
}
