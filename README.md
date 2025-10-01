# Go Neo4j Persistence API (`go-neopersist`)

[![Go Reference](https://pkg.go.dev/badge/github.com/saulfrancisco-ruizacevedo/go-neopersist.svg)](https://pkg.go.dev/github.com/saulfrancisco-ruizacevedo/go-neopersist)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

`go-neopersist` is a reflection-based persistence API for Neo4j, written in Go. It's designed to simplify database interactions by mapping Go structs directly to Neo4j nodes, allowing developers to work with familiar objects instead of writing boilerplate Cypher queries for common operations.

This library is built on top of [`gocypher`](https://github.com/saulfrancisco-ruizacevedo/gocypher), a fluent and programmatic Cypher query builder, providing a powerful two-layer architecture: a high-level OGM (Object-Graph Mapper) for rapid development and a low-level builder for custom, complex queries.

## ✨ Features

* ✅ **Struct-to-Node Mapping**: Use simple Go struct tags (`crud:"..."`) to map your domain models to graph nodes.
* ✅ **Generic Repository**: Get type-safe, out-of-the-box CRUD (`Save`, `FindByID`, `Delete`) operations for any entity.
* ✅ **Centralized Persistence Manager**: An orchestrator for handling complex operations and managing repositories.
* ✅ **Relationship Management**: Easily create relationships between existing nodes with a single method call.
* ✅ **Generic Graph Queries**: Execute any complex Cypher query and receive a generic, frontend-ready graph structure (nodes and edges), perfect for APIs and visualization.
* ✅ **Built on `gocypher`**: Access the underlying fluent query builder for any custom queries your application needs.

## 🚀 Installation

To add `go-neopersist` to your project, run the following command:

```bash
go get github.com/saulfrancisco-ruizacevedo/go-neopersist
```

## Usage & Use Cases

Here’s how to get started with `go-neopersist`.

### 1. Define Your Models with Struct Tags

First, define your domain structs and add `crud` tags to map their fields to node properties. The `pk` tag marks a field as the primary key for `MERGE` and `MATCH` operations.

```go
package models

// User represents a user entity mapped to a :User node.
type User struct {
    UserID string `crud:"pk,property:userId"` // Primary Key, maps to the 'userId' property.
    Name   string `crud:"property:name"`      // Maps to the 'name' property.
}

// Post represents a post entity mapped to a :Post node.
type Post struct {
    PostID string `crud:"pk,property:postId"` // Primary Key, maps to the 'postId' property.
    Title  string `crud:"property:title"`     // Maps to the 'title' property.
}
```

### 2. Set Up the Persistence Manager

The `PersistenceManager` is the main entry point to the library. It holds the database connection and provides access to all features.

```go
package main

import (
    "context"
    "fmt"
    "github.com/saulfrancisco-ruizacevedo/go-neopersist"
)

func main() {
    // Database Configuration
    uri := "neo4j://localhost:7687"
    username := "neo4j"
    password := "your_password"
    dbName := "your_database"
    
    ctx := context.Background()

    // Initialize the real Neo4j driver executor
    dbExecutor, err := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
    if err != nil {
        panic(fmt.Errorf("could not create driver: %w", err))
    }
    defer dbExecutor.Driver.Close(ctx)

    // Verify connectivity to your specific database
    if err := dbExecutor.Verify(ctx); err != nil {
        panic(fmt.Errorf("could not connect to database '%s': %w", dbName, err))
    }
    fmt.Println("Connection successful!")

    // Create the PersistenceManager
    manager := neopersist.NewPersistenceManager(dbExecutor)

    // Now you can use the manager to interact with the database.
}
```
### 3. Use Case: Basic Node CRUD

Use the generic `Repository` for type-safe CRUD operations on a single entity.

```go
// Get a type-safe repository specifically for the User model.
userRepo, err := neopersist.RepositoryFor[models.User](manager)
if err != nil {
    panic(err)
}

// 1. CREATE: Save a new user to the database.
// The Save method performs an "upsert" (MERGE in Cypher).
userToCreate := models.User{UserID: "crud-user-123", Name: "John Doe"}
fmt.Printf("Saving user '%s'...\n", userToCreate.Name)
if err := userRepo.Save(ctx, &userToCreate); err != nil {
    fmt.Printf("Error saving user: %v\n", err)
}
fmt.Println("User saved successfully.")

	// --- 2. READ: Find the newly created user by their ID ---
	// We call the FindByID method, which now executes a query against the database.
	// It returns a pointer to the found entity (*models.User) or an error.
	fmt.Printf("\nSearching for user with ID '%s'...\n", userToCreate.UserID)
	foundUser, err := userRepo.FindByID(ctx, userToCreate.UserID)

	// It is critical to handle the error case first.
	if err != nil {
		// A best practice is to specifically check for the ErrNotFound error.
		// This allows you to distinguish between "not found" and other database failures.
		if errors.Is(err, neopersist.ErrNotFound) {
			fmt.Printf("User with ID '%s' was not found.\n", userToCreate.UserID)
		} else {
			// Handle any other potential errors (e.g., connection issues).
			fmt.Printf("An unexpected error occurred while searching for the user: %v\n", err)
		}
		return // Exit if there was an error.
	}



// 3. DELETE: Remove the user from the database.
// The Delete method uses DETACH DELETE to safely remove the node and its relationships.
fmt.Printf("Deleting user with ID '%s'...\n", userToCreate.UserID)
if err := userRepo.Delete(ctx, "crud-user-123"); err != nil {
    fmt.Printf("Error deleting user: %v\n", err)
}
fmt.Println("User saved successfully.")
```
### 4. Use Case: Creating Relationships

The `PersistenceManager` can orchestrate operations across different entities, like creating a relationship.

```go
// Get repositories for both User and Post
userRepo, _ := neopersist.RepositoryFor[models.User](manager)
postRepo, _ := neopersist.RepositoryFor[models.Post](manager)

// 1. Define the entities we want to connect.
author := models.User{UserID: "author-jane", Name: "Jane Austen"}
book := models.Post{PostID: "book-pride", Title: "Pride and Prejudice"}

// 2. Ensure the nodes exist in the database first.
userRepo.Save(ctx, &author)
postRepo.Save(ctx, &book)

// 3. Use the manager to create a relationship between them.
fmt.Println("Creating WROTE relationship...")
relProps := map[string]interface{}{"year": 1813}
if err := manager.CreateRelation(ctx, &author, &book, "WROTE", relProps); err != nil {
    fmt.Printf("Error creating relationship: %v\n", err)
} else {
    fmt.Println("Relationship created successfully!")
}

// 4. Clean up the created nodes.
userRepo.Delete(ctx, author.UserID)
postRepo.Delete(ctx, book.PostID)
```

## Advanced Usage: Generic Graph Queries

For complex queries that go beyond simple CRUD operations on a single entity, `go-neopersist` provides a powerful `FindGraph` method. This function is designed to execute any query you can build with `gocypher` and return the result in a generic, domain-agnostic graph structure.

This is the ideal solution for:
* Fetching subgraphs with multiple types of nodes and relationships.
* Powering APIs that feed graph visualization libraries (like D3.js or Cytoscape.js).
* Handling complex data retrieval scenarios that don't fit the standard repository pattern.

### Example: Finding a User and Their Posts

In this example, the application builds a custom query to find a specific user and all the posts they `WROTE`. The `PersistenceManager` executes it and returns a `GraphResult` containing all the unique nodes and relationships found.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"

    "[github.com/saulfrancisco-ruizacevedo/go-neopersist](https://github.com/saulfrancisco-ruizacevedo/go-neopersist)"
    "[github.com/saulfrancisco-ruizacevedo/gocypher](https://github.com/saulfrancisco-ruizacevedo/gocypher)"
)

func main() {
    // --- Setup ---
    // ... (database connection setup)
    manager := neopersist.NewPersistenceManager(dbExecutor)
    authorID := "author-123"

    // 1. The client application builds the query logic.
    // This query finds a User, their Posts, and the WROTE relationships.
    queryBuilder := gocypher.NewQueryBuilder().
        Match(gocypher.N("u", "User").WithProperties(map[string]interface{}{"userId": authorID})).
        Match(
            gocypher.NRef("u"), // Reference the user 'u' we already found.
            gocypher.R("r", "WROTE").To(),
            gocypher.N("p", "Post"),
        ).
        // It's crucial to return all elements for the graph.
        Return("u", "r", "p")
        
    // 2. Execute the query using the generic FindGraph method.
    graphResult, err := manager.FindGraph(ctx, queryBuilder)
    if err != nil {
        panic(err)
    }

    // 3. Serialize the result to a frontend-friendly JSON.
    jsonOutput, _ := json.MarshalIndent(graphResult, "", "  ")
    fmt.Println(string(jsonOutput))
}
This produces a clean JSON output that explicitly defines the graphs nodes and edges, ready for any client application to consume:
```json
{
  "nodes": [
    {
      "id": "4:abc:1",
      "labels": ["User"],
      "properties": {
        "name": "George Orwell",
        "userId": "author-123"
      }
    },
    {
      "id": "4:abc:2",
      "labels": ["Post"],
      "properties": {
        "postId": "post-1984",
        "title": "Nineteen Eighty-Four"
      }
    }
  ],
  "edges": [
    {
      "id": "5:def:0",
      "source": "4:abc:1",
      "target": "4:abc:2",
      "type": "WROTE",
      "properties": {
        "year": 1949
      }
    }
  ]
}
```


## 🤝 Contributing

Contributions are welcome! Please feel free to open an issue to discuss a new feature or bug, or submit a pull request.

## 📄 License

This project is licensed under the MIT License. See the `LICENSE` file for details.