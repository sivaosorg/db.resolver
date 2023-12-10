package dbresolver

import (
	"database/sql"
	"sync"

	"github.com/sivaosorg/govm/dbx"
	"github.com/sivaosorg/govm/mysql"
	"github.com/sivaosorg/govm/postgres"
)

// dbConfig represents the common configuration for both Postgres and MySQL.
type dbConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"-"`
	Database string `json:"database"`
}

// DBConnector is an interface for creating and managing database connections.
type DBConnector interface {
	Connect() (*sql.DB, dbx.Dbx)
}

// PostgresConnector implements the DBConnector interface for PostgreSQL.
type PostgresConnector struct {
	Config postgres.PostgresConfig `json:"psql_conf"`
}

// MySQLConnector implements the DBConnector interface for MySQL.
type MySQLConnector struct {
	Config mysql.MysqlConfig `json:"msql_conf"`
}

// MultiTenantDBResolver manages database connections for multiple tenants.
type MultiTenantDBResolver struct {
	connectors map[string]DBConnector
	once       map[string]*sync.Once
}
