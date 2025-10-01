// Package neopersist provides a convenient wrapper around the official Neo4j Go driver
// to simplify database query execution.
package neopersist

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// DBRunner defines the interface for a generic query executor.
// It abstracts the execution of a Cypher query, allowing for different implementations
// or mocking in tests.
type DBRunner interface {
	// Run executes a given Cypher query with parameters and returns a fully-buffered result.
	Run(ctx context.Context, query string, params map[string]interface{}) (*neo4j.EagerResult, error)
}

//---

// Neo4jExecutor is a concrete implementation of the DBRunner interface that uses the
// official Neo4j Go driver. It manages the driver instance and the target database name.
type Neo4jExecutor struct {
	Driver neo4j.DriverWithContext
	DBName string
}

// NewNeo4jExecutor creates and initializes a new Neo4jExecutor.
// It establishes a connection driver with the provided credentials.
//
// Parameters:
//   - uri: The connection URI for the Neo4j instance (e.g., "neo4j://localhost:7687").
//   - username: The username for authentication.
//   - password: The password for authentication.
//   - dbName: The name of the database to connect to (e.g., "neo4j").
//
// Returns:
//
//	A pointer to the newly created Neo4jExecutor or an error if the driver creation fails.
func NewNeo4jExecutor(uri, username, password, dbName string) (*Neo4jExecutor, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("could not create Neo4j driver: %w", err)
	}
	return &Neo4jExecutor{Driver: driver, DBName: dbName}, nil
}

// Verify checks the connectivity to the Neo4j database by running a simple query.
//
// Returns:
//
//	An error if the connection cannot be established or the query fails.
func (e *Neo4jExecutor) Verify(ctx context.Context) error {
	return e.Driver.VerifyConnectivity(ctx)
}

// Run executes a Cypher query using the modern ExecuteQuery function, which handles
// session and transaction management automatically for robust and simple execution.
// This function is suitable for both read and write operations.
//
// Parameters:
//   - ctx: The context for the query execution.
//   - query: The Cypher query string to execute.
//   - params: A map of parameters to be used in the query.
//
// Returns:
//
//	An EagerResult containing all buffered records from the query, or an error if
//	the execution fails.
func (e *Neo4jExecutor) Run(ctx context.Context, query string, params map[string]interface{}) (*neo4j.EagerResult, error) {
	result, err := neo4j.ExecuteQuery(
		ctx,
		e.Driver,
		query,
		params,
		neo4j.EagerResultTransformer, // Buffers all results in memory before returning.
		neo4j.ExecuteQueryWithDatabase(e.DBName),
	)

	if err != nil {
		return nil, fmt.Errorf("error executing neo4j query: %w", err)
	}

	return result, nil
}
