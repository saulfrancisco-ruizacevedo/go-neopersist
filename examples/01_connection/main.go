package main

import (
	"context"
	"fmt"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
)

func main() {
	// --- 1. Database Configuration ---
	// IMPORTANT: Replace with your Neo4j connection details.
	uri := "neo4j://localhost:7687"
	username := "neo4j"
	password := "your_password" // Use your actual password
	dbName := "neo4j"           // The database to use. Default is "neo4j".

	ctx := context.Background()
	fmt.Println("Attempting to connect to Neo4j...")

	// --- 2. Initialize Driver Executor ---
	// NewNeo4jExecutor creates the driver instance but does not verify connectivity yet.
	dbExecutor, err := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
	if err != nil {
		// This error typically occurs if the URI is malformed.
		panic(fmt.Errorf("could not create driver: %w", err))
	}
	// It's crucial to close the driver when the application exits.
	defer dbExecutor.Driver.Close(ctx)

	// --- 3. Verify Connection ---
	// The Verify method checks connectivity against the specific database ('event-weaver').
	// This will fail if the database doesn't exist or credentials are wrong.
	if err := dbExecutor.Verify(ctx); err != nil {
		panic(fmt.Errorf("could not connect to database '%s': %w", dbName, err))
	}

	fmt.Printf("Connection to database '%s' was successful!\n", dbName)
}
