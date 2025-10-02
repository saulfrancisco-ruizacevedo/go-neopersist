package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
	"github.com/saulfrancisco-ruizacevedo/gocypher"
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
	password := "taco1234" // Replace with your password
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
		{UserID: "user-find-1", Name: "Alice", Email: "alice@example.com"},
		{UserID: "user-find-2", Name: "Bob", Email: "bob@example.com"},
		{UserID: "user-find-3", Name: "Charlie", Email: "alice@example.com"}, // Note: Duplicate email
	}
	for _, u := range usersToCreate {
		if err := userRepo.Save(ctx, u); err != nil {
			log.Fatalf("Failed to save sample user %s: %v", u.Name, err)
		}
	}
	fmt.Println("Sample data created successfully.")

	// --- 3. Example: Using FindAll ---
	fmt.Println("\n--- Using FindAll ---")
	allUsers, err := userRepo.FindAll(ctx)
	if err != nil {
		log.Fatalf("Error calling FindAll: %v", err)
	}

	fmt.Printf("FindAll found %d users:\n", len(allUsers))
	for _, user := range allUsers {
		fmt.Printf("  - User: %+v\n", *user)
	}

	// --- 4. Example: Using FindByProperty ---
	fmt.Println("\n--- Using FindByProperty ---")
	emailToFind := "alice@example.com"
	fmt.Printf("Searching for users with email: '%s'\n", emailToFind)
	usersByEmail, err := userRepo.FindByProperty(ctx, "email", emailToFind)
	if err != nil {
		log.Fatalf("Error calling FindByProperty: %v", err)
	}

	fmt.Printf("FindByProperty found %d users:\n", len(usersByEmail))
	for _, user := range usersByEmail {
		fmt.Printf("  - User: %+v\n", *user)
	}

	// --- 5. Example: Using Find with a Custom Query ---
	fmt.Println("\n--- Using Find with a Custom Query ---")
	fmt.Println("Searching for users with name 'Charlie' using a custom query builder...")
	qb := gocypher.NewQueryBuilder().
		Match(gocypher.N("u", "User").WithProperties(map[string]interface{}{"name": "Charlie"})).
		Return("u")

	foundUsers, err := userRepo.Find(ctx, qb)
	if err != nil {
		log.Fatalf("Error calling Find: %v", err)
	}

	fmt.Printf("Custom Find query found %d users:\n", len(foundUsers))
	for _, user := range foundUsers {
		fmt.Printf("  - User: %+v\n", *user)
	}

	// --- 6. Example: Using FindOne for Unique Results ---
	fmt.Println("\n--- Using FindOne for a Unique Result ---")
	// FindOne is perfect for queries that should only ever return a single result,
	// like searching by a unique ID.
	userIdToFind := "user-find-2" // We'll find Bob
	fmt.Printf("Searching for a single user with UserID: '%s'\n", userIdToFind)

	qbOne := gocypher.NewQueryBuilder().
		Match(gocypher.N("u", "User").WithProperties(map[string]interface{}{"userId": userIdToFind})).
		Return("u")

	// The call is simpler: it returns a single user or an error.
	foundUser, err := userRepo.FindOne(ctx, qbOne)
	if err != nil {
		// FindOne automatically handles the "zero results" and "multiple results" cases.
		if errors.Is(err, neopersist.ErrNotFound) {
			fmt.Printf("User with ID '%s' was not found.\n", userIdToFind)
		} else {
			fmt.Printf("An unexpected error occurred: %v\n", err)
		}
	} else {
		// If no error, we have our user directly. No need to check slice length.
		fmt.Printf("FindOne found user: %+v\n", *foundUser)
	}

	// --- 7. Cleanup ---
	fmt.Println("\n--- Cleaning up created users ---")
	for _, u := range usersToCreate {
		if err := userRepo.Delete(ctx, u.UserID); err != nil {
			fmt.Printf("Warning: failed to delete user %s: %v\n", u.UserID, err)
		}
	}
	fmt.Println("Cleanup complete.")
}
