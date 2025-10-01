// Package models contains the domain entities for the application.
// These structs are designed to be used with the neopersist library and
// use struct tags to define their mapping to Neo4j nodes and properties.
package models

// User represents a user entity in the application's domain.
// The `crud` struct tags provide metadata to the neopersist library for
// mapping this struct to a `:User` node in Neo4j.
type User struct {
	// UserID is the unique identifier for the User.
	// The `pk` tag designates it as the primary key for MERGE and MATCH operations.
	// The `property:userId` tag maps this field to the 'userId' property in the database node.
	UserID string `crud:"pk,property:userId"`

	// Name is the display name of the user.
	// The `property:name` tag maps this field to the 'name' property in the database node.
	Name string `crud:"property:name"`
}

// Post represents an article or blog post entity.
// The `crud` struct tags map this struct to a `:Post` node in Neo4j.
type Post struct {
	// PostID is the unique identifier for the Post.
	// The `pk` tag designates it as the primary key.
	// The `property:postId` tag maps this field to the 'postId' property in the database node.
	PostID string `crud:"pk,property:postId"`

	// Title is the title of the post.
	// The `property:title` tag maps this field to the 'title' property in the database node.
	Title string `crud:"property:title"`
}
