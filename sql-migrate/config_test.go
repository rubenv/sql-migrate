package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func generateFile(lines []string) (string, error) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		file.Close()
	}()

	for _, l := range lines {
		file.WriteString(fmt.Sprintf("%s\n", l))
	}

	return file.Name(), nil
}

func cleanup(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			return err
		}
	}

	return nil
}

func TestReadConfig(t *testing.T) {
	// Bad lines
	lines := []string{
		"development:",
		"  dialect: postgres",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  dialect: postgres",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		" schema: public", // Bad line
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err := generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	_, err = ReadConfig()
	require.Error(t, err)

	// Good config
	lines = []string{
		"development:",
		"  dialect: postgres",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  dialect: postgres",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	envMap, err := ReadConfig()
	if err != nil {
		t.Error(err)
	}

	require.Equal(t, 2, len(envMap))
}

func TestGetEnvironment(t *testing.T) {
	// Bad lines - can't find key
	lines := []string{
		"development:",
		"  dialect: postgres",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  dialect: postgres",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		" schema: public", // Bad line
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err := generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	_, err = GetEnvironment()
	require.ErrorContains(t, err, "yaml: line 12: did not find expected key")

	// Parseable - unmatched environment
	lines = []string{
		"development:",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "foobar"
	_, err = GetEnvironment()
	require.ErrorContains(t, err, "no environment: foobar")

	// Error - missing dialect
	lines = []string{
		"development:",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "development"
	_, err = GetEnvironment()
	require.ErrorContains(t, err, "no dialect")

	// Error - missing datasource
	lines = []string{
		"development:",
		"  dialect: postgres",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: false",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "development"
	_, err = GetEnvironment()
	require.ErrorContains(t, err, "no data source")

	// No migration dir
	lines = []string{
		"development:",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dialect: postgres",
		"  schema: public",
		"  table: migrations",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "development"
	env, err := GetEnvironment()
	require.Equal(t, "migrations", env.Dir)

	// Test setting table and absent limitprecision
	lines = []string{
		"development:",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dialect: postgres",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "development"
	env, err = GetEnvironment()
	require.Equal(t, "migrations", env.TableName)
	require.False(t, env.LimitTimePrecision)

	// Test setting table and absent limitprecision
	lines = []string{
		"development:",
		"  datasource: host=127.0.0.1 dbname=reporting user=root password=root sslmode=disable",
		"  dialect: postgres",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
		"  limitprecision: true",
		"",
		"docker:",
		"  datasource: host=portal_db dbname=reporting user=root password=root sslmode=disable",
		"  dir: my_db/migrations",
		"  schema: public",
		"  table: migrations",
	}
	configFile, err = generateFile(lines)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		err = cleanup(configFile)
		if err != nil {
			t.Error(err)
		}
	}()

	ConfigFile = configFile
	ConfigEnvironment = "development"
	env, err = GetEnvironment()
	require.True(t, env.LimitTimePrecision)
}
