package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
	"github.com/saulfrancisco-ruizacevedo/go-neopersist/examples/models"
)

func main() {
	// --- Setup (Same as the connection example) ---
	uri, username, password, dbName := "neo4j://localhost:7687", "neo4j", "your_password", "neo4j"
	ctx := context.Background()

	dbExecutor, err := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
	if err != nil {
		panic(err)
	}
	defer dbExecutor.Driver.Close(ctx)
	if err := dbExecutor.Verify(ctx); err != nil {
		panic(err)
	}

	// --- Initialize PersistenceManager and Repository ---
	manager := neopersist.NewPersistenceManager(dbExecutor)

	// Get a type-safe repository specifically for the User model.
	userRepo, err := neopersist.RepositoryFor[models.User](manager)
	if err != nil {
		panic(err)
	}

	fmt.Println("\n--- Running Node CRUD Operations ---")
	userToCreate := models.User{UserID: "crud-user-123", Name: "John Doe"}

	// 1. CREATE: Save the new user to the database.
	// The Save method performs an "upsert" (MERGE in Cypher).
	fmt.Printf("Saving user '%s'...\n", userToCreate.Name)
	if err := userRepo.Save(ctx, &userToCreate); err != nil {
		fmt.Printf("Error saving user: %v\n", err)
		return
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

	// If err is nil, the operation was successful and foundUser contains the data.
	fmt.Println("User found successfully!")
	// We dereference the pointer (*) to print the contents of the struct.
	// The '%+v' format verb includes field names for clarity.
	fmt.Printf("Found User Details: %+v\n", *foundUser)

	// 3. DELETE: Remove the user from the database.
	// The Delete method uses DETACH DELETE to safely remove the node and its relationships.
	fmt.Printf("\nDeleting user with ID '%s'...\n", userToCreate.UserID)
	if err := userRepo.Delete(ctx, userToCreate.UserID); err != nil {
		fmt.Printf("Error deleting user: %v\n", err)
		return
	}
	fmt.Println("User deleted successfully.")
}
