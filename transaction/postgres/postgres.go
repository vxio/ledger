package postgres

import (
	"flag"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/peterbourgon/ff"
)

type Config struct {
	Host         string
	Port         int
	User         string
	DatabaseName string
}

// connect to Postgres and return a database handle representing a pool of connections
func Connect(config *Config) (*sqlx.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable",
		config.Host,
		config.Port,
		config.User,
		config.DatabaseName,
	)

	db, err := sqlx.Connect("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %v", err)
	}

	err = setup(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Parse the flags in the flag set from the command line.
// Additional options may be provided to parse from environment variables, but flags get priority.
//
// Example .env file
// 	POSTGRES_HOST=localhost
// 	POSTGRES_PORT=5432
// 	POSTGRES_USER=alice
// 	POSTGRES_DB_NAME=transactions_dev
func Parse() (*Config, error) {
	var err error

	postgresFlags := flag.NewFlagSet("postgres", flag.ExitOnError)
	var (
		host   = postgresFlags.String("host", "localhost", "host to connect to")
		port   = postgresFlags.Int("port", 5432, "port to bind to")
		user   = postgresFlags.String("user", "", "user to sign in as")
		dbName = postgresFlags.String("db_name", "", "name of the database")
		// ignore flag passed by Intellij since ff package doesn't have option to ignore undefined flags
		_ = postgresFlags.String("test.v", "", "")
	)

	err = ff.Parse(postgresFlags, os.Args[1:],
		ff.WithIgnoreUndefined(true),
		ff.WithEnvVarPrefix("POSTGRES"),
	)
	if err != nil {
		return nil, err
	}

	return &Config{
		Host:         *host,
		Port:         *port,
		User:         *user,
		DatabaseName: *dbName,
	}, nil
}

// configures the database settings
func setup(db *sqlx.DB) error {
	// install extension for creating UUIDs
	_, err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
	if err != nil {
		return fmt.Errorf("adding UUID extension: %v", err)
	}

	// set default timezone to UTC
	_, err = db.Exec("SET timezone to 'UTC'")
	if err != nil {
		return fmt.Errorf("setting database default timezone: %v", err)
	}

	err = createTransactionsTable(db)
	if err != nil {
		return fmt.Errorf("creating db tables: %v", err)
	}

	return nil
}
