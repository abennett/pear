package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// requires postgres 11

const (
	createSeedTable = `
CREATE TABLE IF NOT EXISTS seed (
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	sower TEXT NOT NULL,
	topic TEXT NOT NULL,
	planted TIMESTAMPTZ NOT NULL DEFAULT now()
)`

	createPearTable = `
CREATE TABLE IF NOT EXISTS pear(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	seed_id INTEGER REFERENCES seed (id) NOT NULL UNIQUE,
	picker TEXT NOT NULL,
	picked TIMESTAMPTZ NOT NULL DEFAULT now()
)`
)

func InitPG(url string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		return nil, err
	}
	db.MustExec(createSeedTable)
	db.MustExec(createPearTable)
	return db, nil
}
