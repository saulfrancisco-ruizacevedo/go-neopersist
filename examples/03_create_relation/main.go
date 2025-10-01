package main

import (
	"context"
	"fmt"

	"github.com/saulfrancisco-ruizacevedo/go-neopersist"
	"github.com/saulfrancisco-ruizacevedo/go-neopersist/examples/models"
)

func main() {
	// --- Setup ---
	uri, username, password, dbName := "neo4j://localhost:7687", "neo4j", "your_password", "neo4j"
	ctx := context.Background()
	dbExecutor, _ := neopersist.NewNeo4jExecutor(uri, username, password, dbName)
	defer dbExecutor.Driver.Close(ctx)
	dbExecutor.Verify(ctx)

	// --- Initialize Manager and Repositories ---
	manager := neopersist.NewPersistenceManager(dbExecutor)
	userRepo, _ := neopersist.RepositoryFor[models.User](manager)
	postRepo, _ := neopersist.RepositoryFor[models.Post](manager)

	fmt.Println("\n--- Creating Relationship ---")
	// 1. Define the entities we want to connect.
	author := models.User{UserID: "author-jane", Name: "Jane Austen"}
	book := models.Post{PostID: "book-pride", Title: "Pride and Prejudice"}

	// 2. Ensure the nodes exist in the database first.
	fmt.Println("Creating nodes...")
	userRepo.Save(ctx, &author)
	postRepo.Save(ctx, &book)
	fmt.Println("Nodes created.")

	// 3. Use the manager to create a relationship between them.
	// The manager uses reflection on the instances to find their labels and primary keys.
	fmt.Println("Creating WROTE relationship...")
	relProps := map[string]interface{}{"year": 1813}
	if err := manager.CreateRelation(ctx, &author, &book, "WROTE", relProps); err != nil {
		fmt.Printf("Error creating relationship: %v\n", err)
	} else {
		fmt.Println("Relationship created successfully!")
	}

	// 4. Clean up the created nodes.
	fmt.Println("\nCleaning up...")
	userRepo.Delete(ctx, author.UserID)
	postRepo.Delete(ctx, book.PostID)
	fmt.Println("Cleanup complete.")
}
