// This example demonstrates how to use the generic FindGraph method of the PersistenceManager
// to execute a custom, user-defined graph query and serialize the result to a JSON format
// suitable for frontend applications or other services.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
	"github.com/saulfrancisco-ruizacevedo/gocypher"
)

func main() {
	// --- 1. Database Setup ---
	// It's recommended to load these values from a configuration file or environment variables.
	uri := "neo4j://localhost:7687"
	username := "neo4j"
	password := "your_password" // Replace with your actual password
	dbName := "neo4j"
	ctx := context.Background()

	// Initialize the executor which manages the database driver.
	dbExecutor, err := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
	if err != nil {
		log.Fatalf("Fatal: Could not create database executor: %v", err)
	}
	defer dbExecutor.Driver.Close(ctx)

	// Verify the connection to ensure credentials and URI are correct.
	if err := dbExecutor.Verify(ctx); err != nil {
		log.Fatalf("Fatal: Could not connect to database: %v", err)
	}

	// The PersistenceManager provides high-level database operations.
	manager := neopersist.NewPersistenceManager(dbExecutor)

	// --- 2. Build a Custom Graph Query ---
	fmt.Println("\n--- Building a custom graph query ---")
	authorID := "author-graph-1" // The ID of the root node for our graph search.

	// The client application is responsible for defining the exact shape of the graph
	// it wants to retrieve. Here, we look for a specific author and the posts they wrote.
	queryBuilder := gocypher.NewQueryBuilder().
		// Start by matching the root of our graph: a specific User node.
		Match(gocypher.N("u", "User").WithProperties(map[string]interface{}{"userId": authorID})).
		// Then, match the pattern of that user writing a post.
		Match(
			gocypher.NRef("u"), // Reference the user 'u' we already found.
			gocypher.R("r", "WROTE").To(),
			gocypher.N("p", "Post"),
		).
		// It is crucial to return all elements (nodes and relationships) that
		// you want to be included in the final GraphResult struct.
		Return("u", "r", "p")

	// --- 3. Execute the Generic Graph Query ---
	fmt.Printf("--- Fetching Graph for User ID '%s' ---\n", authorID)

	// Pass the fully constructed query builder to the manager's FindGraph method.
	// FindGraph is domain-agnostic; it will execute any valid query and map the results.
	graphResult, err := manager.FindGraph(ctx, queryBuilder)
	if err != nil {
		// This could be ErrNotFound or any other database error.
		log.Fatalf("Error fetching graph: %v", err)
	}

	// --- 4. Process and Display the Result ---
	// The GraphResult struct is designed to be easily serialized to JSON.
	// We use MarshalIndent for pretty-printing the output.
	jsonOutput, err := json.MarshalIndent(graphResult, "", "  ")
	if err != nil {
		log.Fatalf("Error serializing result to JSON: %v", err)
	}

	fmt.Println("\n--- Generic Graph JSON Output for Frontend ---")
	fmt.Println(string(jsonOutput))
}
