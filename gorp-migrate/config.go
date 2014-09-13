package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/coopernurse/gorp"
	"gopkg.in/yaml.v1"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var dialects = map[string]gorp.Dialect{
	"sqlite3":  gorp.SqliteDialect{},
	"postgres": gorp.PostgresDialect{},
	"mysql":    gorp.MySQLDialect{},
}

var ConfigFile string
var ConfigEnvironment string

func ConfigFlags(f *flag.FlagSet) {
	f.StringVar(&ConfigFile, "config", "dbconfig.yml", "Configuration file to use.")
	f.StringVar(&ConfigEnvironment, "env", "production", "Environment to use.")
}

type Environment struct {
	Dialect    string `yaml:"dialect"`
	DataSource string `yaml:"datasource"`
	Dir        string `yaml:"dir"`
}

func ReadConfig() (map[string]*Environment, error) {
	file, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		return nil, err
	}

	config := make(map[string]*Environment)
	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func GetEnvironment() (*Environment, error) {
	config, err := ReadConfig()
	if err != nil {
		return nil, err
	}

	env := config[ConfigEnvironment]
	if env == nil {
		return nil, errors.New("No environment: " + ConfigEnvironment)
	}

	if env.Dialect == "" {
		return nil, errors.New("No dialect specified")
	}

	if env.DataSource == "" {
		return nil, errors.New("No data source specified")
	}

	if env.Dir == "" {
		env.Dir = "migrations"
	}

	return env, nil
}

func GetConnection(env *Environment) (*gorp.DbMap, error) {
	db, err := sql.Open(env.Dialect, env.DataSource)
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to database: %s", err)
	}

	dialect, exists := dialects[env.Dialect]
	if !exists {
		return nil, fmt.Errorf("Unsupported dialect: %s", env.Dialect)
	}

	dbmap := &gorp.DbMap{Db: db, Dialect: dialect}
	return dbmap, nil
}
