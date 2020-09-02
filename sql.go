package main

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/stub"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/hashicorp/go-hclog"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

// requires postgres 11

func InitPG(url string, logger hclog.Logger) (*sqlx.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}
	dbx := sqlx.NewDb(db, "pgx")
	logger.Debug("migrating db")
	if err = migrateDB(dbx); err != nil {
		return nil, fmt.Errorf("failed migrating the db: %w", err)
	}
	return dbx, nil
}

func migrateDB(db *sqlx.DB) error {
	driver, err := stub.WithInstance(db.DB, &stub.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance("file://sql", "db", driver)
	if err != nil {
		return err
	}
	if err = m.Up(); err == migrate.ErrNoChange {
		return nil
	}
	return err
}
