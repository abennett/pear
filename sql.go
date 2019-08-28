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
	sower TEXT not null,
	topic TEXT not null,
	planted TIMESTAMPTZ not null
)`

	createPearTable = `
CREATE TABLE IF NOT EXISTS pear(
	id INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
	seed_id INTEGER REFERENCES seed (id) NOT NULL,
	picker TEXT not null,
	picked TIMESTAMPTZ not null
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
