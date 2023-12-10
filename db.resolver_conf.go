package dbresolver

import (
	"database/sql"
	"sync"

	"github.com/sivaosorg/govm/dbx"
)

var (
	dbs = make(map[string]struct {
		C *sql.DB
		S dbx.Dbx
	})
	mu            sync.RWMutex
	defaultConfig = dbConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "default-db-user",
		Password: "default-db-password",
		Database: "default-db-name",
	}
)
