package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/go-gorp/gorp/v3"
	"gopkg.in/yaml.v2"

	migrate "github.com/rubenv/sql-migrate"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var dialects = map[string]gorp.Dialect{
	"sqlite3":  gorp.SqliteDialect{},
	"postgres": gorp.PostgresDialect{},
	"mysql":    gorp.MySQLDialect{Engine: "InnoDB", Encoding: "UTF8"},
}

var (
	ConfigFile        string
	ConfigEnvironment string
)

func ConfigFlags(f *flag.FlagSet) {
	f.StringVar(&ConfigFile, "config", "dbconfig.yml", "Configuration file to use.")
	f.StringVar(&ConfigEnvironment, "env", "development", "Environment to use.")
}

type Environment struct {
	Dialect       string    `yaml:"dialect"`
	DataSource    string    `yaml:"datasource"`
	Dir           string    `yaml:"dir"`
	TableName     string    `yaml:"table"`
	SchemaName    string    `yaml:"schema"`
	IgnoreUnknown bool      `yaml:"ignoreunknown"`
	Tls           TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	CA   string `yaml:"ca"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

func ReadConfig() (map[string]*Environment, error) {
	file, err := os.ReadFile(ConfigFile)
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
	env.DataSource = os.ExpandEnv(env.DataSource)

	env.Tls.CA = os.ExpandEnv(env.Tls.CA)
	env.Tls.Cert = os.ExpandEnv(env.Tls.Cert)
	env.Tls.Key = os.ExpandEnv(env.Tls.Key)

	if env.Dir == "" {
		env.Dir = "migrations"
	}

	if env.TableName != "" {
		migrate.SetTable(env.TableName)
	}

	if env.SchemaName != "" {
		migrate.SetSchema(env.SchemaName)
	}

	migrate.SetIgnoreUnknown(env.IgnoreUnknown)

	return env, nil
}

func GetConnection(env *Environment) (*sql.DB, string, error) {
	// Load CA cert for RDS Aurora MySQL if tls specified
	if env.Dialect == "mysql" && isTlsCustomEnabled(env) {
		if env.Tls.CA == "" {
			return nil, "", errors.New("file CA is not set")
		}

		err := RegisterTlsConfig(env.Tls.CA, "custom")
		if err != nil {
			return nil, "", fmt.Errorf("cannot register TLS config: %w", err)
		}
	}

	db, err := sql.Open(env.Dialect, env.DataSource)
	if err != nil {
		return nil, "", fmt.Errorf("Cannot connect to database: %w", err)
	}

	// Make sure we only accept dialects that were compiled in.
	_, exists := dialects[env.Dialect]
	if !exists {
		return nil, "", fmt.Errorf("Unsupported dialect: %s", env.Dialect)
	}

	return db, env.Dialect, nil
}

// GetVersion returns the version.
func GetVersion() string {
	if buildInfo, ok := debug.ReadBuildInfo(); ok && buildInfo.Main.Version != "(devel)" {
		return buildInfo.Main.Version
	}
	return "dev"
}

func RegisterTlsConfig(pemPath, tlsType string) (err error) {
	caCertPool := x509.NewCertPool()
	pem, err := os.ReadFile(pemPath)
	if err != nil {
		return err
	}

	if ok := caCertPool.AppendCertsFromPEM(pem); !ok {
		return fmt.Errorf("cannot append certs from PEM")
	}

	mysql.RegisterTLSConfig(tlsType, &tls.Config{
		RootCAs: caCertPool,
	})

	return nil
}

func isTlsCustomEnabled(env *Environment) bool {
	return strings.Contains(env.DataSource, "tls=custom")
}
