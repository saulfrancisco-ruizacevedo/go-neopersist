package main

import (
	"context"
	"fmt"
	"log"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
)

// Define a sample User model with the required struct tags.
type User struct {
	UserID string `crud:"pk,property:userId"`
	Name   string `crud:"property:name"`
	Email  string `crud:"property:email"`
}

func main() {
	// --- 1. Database Setup ---
	uri := "neo4j://localhost:7687"
	username := "neo4j"
	password := "your_password" // Replace with your password
	dbName := "neo4j"
	ctx := context.Background()

	dbExecutor, err := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
	if err != nil {
		log.Fatalf("Fatal: Could not create executor: %v", err)
	}
	defer dbExecutor.Driver.Close(ctx)

	manager := neopersist.NewPersistenceManager(dbExecutor)
	userRepo, err := neopersist.RepositoryFor[User](manager)
	if err != nil {
		log.Fatalf("Fatal: Could not create repository: %v", err)
	}

	// --- 2. Prepare Sample Data ---
	fmt.Println("--- Preparing sample data ---")
	usersToCreate := []*User{
		{UserID: "user-count-1", Name: "Alice", Email: "alice@test.com"},
		{UserID: "user-count-2", Name: "Bob", Email: "bob@test.com"},
		{UserID: "user-count-3", Name: "Alice", Email: "alice-2@test.com"},
	}
	for _, u := range usersToCreate {
		if err := userRepo.Save(ctx, u); err != nil {
			log.Fatalf("Failed to save sample user %s: %v", u.Name, err)
		}
	}
	fmt.Println("Sample data created successfully.")

	// --- 3. Example: Using Count ---
	fmt.Println("\n--- Using Count to get the total number of users ---")
	totalUsers, err := userRepo.Count(ctx)
	if err != nil {
		log.Fatalf("Error calling Count: %v", err)
	}
	fmt.Printf("Total number of users found: %d\n", totalUsers)

	// --- 4. Example: Using CountByProperty ---
	fmt.Println("\n--- Using CountByProperty to count users with a specific name ---")
	nameToCount := "Alice"
	countOfAlices, err := userRepo.CountByProperty(ctx, "name", nameToCount)
	if err != nil {
		log.Fatalf("Error calling CountByProperty: %v", err)
	}
	fmt.Printf("Number of users with the name '%s': %d\n", nameToCount, countOfAlices)

	// --- 5. Cleanup ---
	fmt.Println("\n--- Cleaning up created users ---")
	for _, u := range usersToCreate {
		if err := userRepo.Delete(ctx, u.UserID); err != nil {
			fmt.Printf("Warning: failed to delete user %s: %v\n", u.UserID, err)
		}
	}
	fmt.Println("Cleanup complete.")
}
