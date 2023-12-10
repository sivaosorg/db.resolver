package example

import (
	dbresolver "github.com/sivaosorg/db.resolver"
	"github.com/sivaosorg/govm/logger"
	"github.com/sivaosorg/govm/postgres"
)

func main() {
	config_1 := postgres.GetPostgresConfigSample().
		SetEnabled(true).
		SetDatabase("your_db").
		SetPort(6666).
		SetPassword("@@@@@@@@").
		SetUsername("XXXX").
		SetDebugMode(false)

	config_2 := postgres.GetPostgresConfigSample().
		SetEnabled(true).
		SetDatabase("your_db").
		SetPort(6666).
		SetPassword("@@@@@@@@").
		SetUsername("XXXX").
		SetDebugMode(false)

	postgresConfigs := []postgres.MultiTenantPostgresConfig{
		{Key: "postgres_tenant_1", Config: *postgres.GetPostgresConfigSample()},
		{Key: "postgres_tenant_2", Config: *postgres.GetPostgresConfigSample()},
	}

	connector1 := dbresolver.NewPostgresConnector(*config_1)
	connector2 := dbresolver.NewPostgresConnector(*config_2)

	resolver := dbresolver.NewMultiTenantDBResolver()

	resolver.AddConnector("psql_node1", connector1).AddConnector("psql_node2", connector2).AddPsqlConnectors(postgresConfigs...)

	_, s1 := resolver.GetConnector("psql_node1")

	logger.Infof("Conn status node1 = %v", s1.Json())

	_, s2 := resolver.GetConnector("psql_node2")

	logger.Infof("Conn status node2 = %v", s2.Json())

	_, s := resolver.GetConnector("psql_node1")

	logger.Infof("Conn status node1 retake = %v", s.Json())
}
