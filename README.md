# sql-migrate

> SQL Schema migration tool for [Go](http://golang.org/). Based on [gorp](https://github.com/coopernurse/gorp) and [goose](https://bitbucket.org/liamstask/goose).

[![Build Status](https://travis-ci.org/rubenv/sql-migrate.svg?branch=master)](https://travis-ci.org/rubenv/sql-migrate) [![GoDoc](https://godoc.org/github.com/rubenv/sql-migrate?status.png)](https://godoc.org/github.com/rubenv/sql-migrate)

## Features

* Usable as a CLI tool or as a library
* Supports SQLite, PostgreSQL, MySQL, MSSQL and Oracle databases (through [gorp](https://github.com/coopernurse/gorp))
* Can embed migrations into your application
* Migrations are defined with SQL for full flexibility
* Atomic migrations
* Up/down migrations to allow rollback
* Supports multiple database types in one project

## Installation

To install the library and command line program, use the following:

```bash
go get github.com/rubenv/sql-migrate/...
```

## Usage
### As a standalone tool
```
$ sql-migrate --help
usage: sql-migrate [--version] [--help] <command> [<args>]

Available commands are:
    down      Undo a database migration
    redo      Reapply the last migration
    status    Show migration status
    up        Migrates the database to the most recent version available
```

Each command requires a configuration file (which defaults to `dbconfig.yml`, but can be specified with the `-config` flag). This config file should specify one or more environments:

```yml
development:
    dialect: sqlite3
    datasource: test.db
    dir: migrations/sqlite3

production:
    dialect: postgres
    datasource: dbname=myapp sslmode=disable
    dir: migrations/postgres
```

The environment that will be used can be specified with the `-env` flag (defaults to `development`).

### As a library
TODO

## Writing migrations

## Embedding migrations with [bindata](https://github.com/jteeuwen/go-bindata)
TODO

## License 

    (The MIT License)

    Copyright (C) 2014 by Ruben Vermeersch <ruben@rocketeer.be>

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"), to deal
    in the Software without restriction, including without limitation the rights
    to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
    copies of the Software, and to permit persons to whom the Software is
    furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included in
    all copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
    AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
    OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
    THE SOFTWARE.