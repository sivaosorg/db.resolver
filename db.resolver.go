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
		dbs: make(map[string]struct {
			C *sql.DB
			S dbx.Dbx
		}),
	}
}

// AddConnector adds a new database connector for a specific tenant.
func (r *MultiTenantDBResolver) AddConnector(tenantId string, connector DBConnector) *MultiTenantDBResolver {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[tenantId] = connector
	r.once[tenantId] = &sync.Once{}
	return r
}

func (r *MultiTenantDBResolver) AddConnectors(tenantId string, connectors ...DBConnector) *MultiTenantDBResolver {
	r.mu.Lock()
	defer r.mu.Unlock()
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

// AddConnectorsFromConfig adds database connectors from configuration data.
func (r *MultiTenantDBResolver) AddConnectorsFromConfig(configs ...interface{}) *MultiTenantDBResolver {
	for _, config := range configs {
		switch c := config.(type) {
		case postgres.MultiTenantPostgresConfig:
			connector := NewPostgresConnector(c.Config)
			r.AddConnector(c.Key, connector)
		case mysql.MultiTenantMysqlConfig:
			connector := NewMySQLConnector(c.Config)
			r.AddConnector(c.Key, connector)
		case postgres.ClusterMultiTenantPostgresConfig:
			for _, clusterConfig := range c.Clusters {
				connector := NewPostgresConnector(clusterConfig.Config)
				r.AddConnector(clusterConfig.Key, connector)
			}
		case mysql.ClusterMultiTenantMysqlConfig:
			for _, clusterConfig := range c.Clusters {
				connector := NewMySQLConnector(clusterConfig.Config)
				r.AddConnector(clusterConfig.Key, connector)
			}
		}
	}
	return r
}

// GetConnector returns a database connection for a specific tenant.
func (r *MultiTenantDBResolver) GetConnector(tenantId string) (*sql.DB, dbx.Dbx) {
	r.mu.RLock()
	connector, ok := r.connectors[tenantId]
	once := r.once[tenantId]
	r.mu.RUnlock()

	if !ok {
		r.mu.Lock()
		defer r.mu.Unlock()
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
		r.dbs[tenantId] = struct {
			C *sql.DB
			S dbx.Dbx
		}{
			C: db,
			S: s,
		}
	})
	conn := r.dbs[tenantId]
	return conn.C, conn.S
}

// RemoveConnector removes a database connector for a specific tenant.
func (r *MultiTenantDBResolver) RemoveConnector(tenantId string) *MultiTenantDBResolver {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.connectors, tenantId)
	delete(r.once, tenantId)
	delete(r.dbs, tenantId)
	return r
}

// UpdateConnector updates an existing database connector for a specific tenant.
func (r *MultiTenantDBResolver) UpdateConnector(tenantId string, connector DBConnector) *MultiTenantDBResolver {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors[tenantId] = connector
	r.once[tenantId] = &sync.Once{}
	return r
}

// CloseConnection closes all database connections for a specific tenant.
func (r *MultiTenantDBResolver) CloseConnection(tenantId string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, ok := r.dbs[tenantId]; ok {
		if conn.C != nil {
			conn.C.Close()
		}
		delete(r.dbs, tenantId)
	}
}

// CloseAllConnections closes all database connections for all tenants.
func (r *MultiTenantDBResolver) CloseAllConnections() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tenantId, conn := range r.dbs {
		if conn.C != nil {
			conn.C.Close()
		}
		delete(r.dbs, tenantId)
	}
}

// ClearAllConnectors removes all connectors and closes associated connections.
func (r *MultiTenantDBResolver) ClearAllConnectors() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tenantId := range r.connectors {
		r.CloseConnection(tenantId)
		delete(r.connectors, tenantId)
		delete(r.once, tenantId)
	}
}

// SetDefaultConnector sets a default database connector for cases where no specific connector is available.
func (r *MultiTenantDBResolver) SetDefaultConnector(connector DBConnector) *MultiTenantDBResolver {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectors["default"] = connector
	r.once["default"] = &sync.Once{}
	return r
}

// GetDefaultConnector returns the default database connector.
func (r *MultiTenantDBResolver) GetDefaultConnector() (*sql.DB, dbx.Dbx) {
	return r.GetConnector("default")
}

// GetConnectorInfo returns information about a specific connector for a tenant.
func (r *MultiTenantDBResolver) GetConnectorInfo(tenantId string) (DBConnector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	connector, ok := r.connectors[tenantId]
	return connector, ok
}

// HealthCheck performs a health check on all connectors and returns the status for each tenant.
func (r *MultiTenantDBResolver) HealthCheck() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var wg sync.WaitGroup
	status := make(map[string]bool, len(r.connectors))
	for tenantId, connector := range r.connectors {
		wg.Add(1)
		go func(t string, c DBConnector) {
			defer wg.Done()
			_, s := c.Connect()
			status[t] = s.IsConnected
		}(tenantId, connector)
	}
	wg.Wait()
	return status
}

// SafeConnector executes a function that requires a database connection safely.
// The function receives the database connection and returns an error if any.
// The connection is automatically closed after the function execution.
func (r *MultiTenantDBResolver) SafeConnector(tenantId string, fn func(db *sql.DB) error) error {
	db, _ := r.GetConnector(tenantId)
	defer r.CloseConnection(tenantId)
	return fn(db)
}

// RefreshConnector refreshes the database connection for a specific tenant.
func (r *MultiTenantDBResolver) RefreshConnector(tenantId string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.once[tenantId]; ok {
		r.once[tenantId] = &sync.Once{}
	}
}
