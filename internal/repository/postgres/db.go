package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func New(dsn string) (*sqlx.DB, error) {
	return sqlx.Connect("pgx", dsn)
}