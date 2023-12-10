package dbresolver

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/sivaosorg/govm/dbx"
	"github.com/sivaosorg/govm/logger"
	"github.com/sivaosorg/govm/mysql"
	"github.com/sivaosorg/govm/postgres"
	"github.com/sivaosorg/mysqlconn"
	"github.com/sivaosorg/postgresconn"
)

// NewPostgresConnector creates a new PostgresConnector instance.
func NewPostgresConnector(config postgres.PostgresConfig) *PostgresConnector {
	return &PostgresConnector{Config: config}
}

// NewMySQLConnector creates a new MySQLConnector instance.
func NewMySQLConnector(config mysql.MysqlConfig) *MySQLConnector {
	return &MySQLConnector{Config: config}
}

func (p *PostgresConnector) Connect() (*sql.DB, dbx.Dbx) {
	psql, s := postgresconn.NewClient(p.Config)
	if s.IsConnected {
		return psql.GetConn().DB, s
	}
	return nil, s
}

func (m *MySQLConnector) Connect() (*sql.DB, dbx.Dbx) {
	msql, s := mysqlconn.NewClient(m.Config)
	return msql.GetConn(), s
}

// NewMultiTenantDBResolver creates a new MultiTenantDBResolver instance.
func NewMultiTenantDBResolver() *MultiTenantDBResolver {
	return &MultiTenantDBResolver{
		connectors: make(map[string]DBConnector),
		once:       make(map[string]*sync.Once),
	}
}

// AddConnector adds a new database connector for a specific tenant.
func (r *MultiTenantDBResolver) AddConnector(tenantId string, connector DBConnector) *MultiTenantDBResolver {
	mu.Lock()
	defer mu.Unlock()
	r.connectors[tenantId] = connector
	r.once[tenantId] = &sync.Once{}
	return r
}

func (r *MultiTenantDBResolver) AddConnectors(tenantId string, connectors ...DBConnector) *MultiTenantDBResolver {
	mu.Lock()
	defer mu.Unlock()
	for _, connector := range connectors {
		r.connectors[tenantId] = connector
		r.once[tenantId] = &sync.Once{}
	}
	return r
}

func (r *MultiTenantDBResolver) AddPsqlConnectors(connectors ...postgres.MultiTenantPostgresConfig) *MultiTenantDBResolver {
	for _, connector := range connectors {
		r.AddConnector(connector.Key, NewPostgresConnector(connector.Config))
	}
	return r
}

func (r *MultiTenantDBResolver) AddMsqlConnectors(connectors ...mysql.MultiTenantMysqlConfig) *MultiTenantDBResolver {
	for _, connector := range connectors {
		r.AddConnector(connector.Key, NewMySQLConnector(connector.Config))
	}
	return r
}

// GetConnector returns a database connection for a specific tenant.
func (r *MultiTenantDBResolver) GetConnector(tenantId string) (*sql.DB, dbx.Dbx) {
	mu.RLock()
	connector, ok := r.connectors[tenantId]
	once := r.once[tenantId]
	mu.RUnlock()

	if !ok {
		mu.Lock()
		defer mu.Unlock()
		// Check again to avoid race condition
		if connector, ok = r.connectors[tenantId]; !ok {
			message := fmt.Sprintf("No connector found for tenant %s", tenantId)
			logger.Warnf(message)
			s := dbx.NewDbx().SetConnected(false).SetMessage(message).SetNewInstance(false).SetPid(0).SetDebugMode(true)
			return nil, *s
		}
	}

	// This will be executed only once for the first connection
	once.Do(func() {
		start := time.Now()
		db, s := connector.Connect()
		if !s.IsConnected {
			logger.Errorf(fmt.Sprintf("Error initializing database connection for tenant %s (executed in %v): %s", tenantId, time.Since(start), s.Message), s.Error)
		}
		dbs[tenantId] = struct {
			C *sql.DB
			S dbx.Dbx
		}{
			C: db,
			S: s,
		}
	})
	conn, _ := dbs[tenantId]
	return conn.C, conn.S
}
