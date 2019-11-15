package main

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// requires postgres 11

func InitPG(url string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}
