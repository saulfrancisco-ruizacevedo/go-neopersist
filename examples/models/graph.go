// Package models contains the domain entities and data transfer objects for the application.
// The structs in this file are specifically designed to represent a generic graph structure,
// making them ideal for serializing query results to JSON for frontend clients or other services.
package models

// GraphNode represents a generic node from a Neo4j graph.
// It is a domain-agnostic representation, capturing the essential components of any node:
// its unique internal ID, its labels, and its properties. This struct is designed to be
// easily serialized to JSON.
type GraphNode struct {
	// ID is the unique internal identifier assigned by Neo4j to the node (ElementId).
	ID string `json:"id"`

	// Labels is a slice of strings containing all the labels attached to the node (e.g., ["User", "Person"]).
	Labels []string `json:"labels"`

	// Properties is a map containing the key-value properties of the node.
	Properties map[string]interface{} `json:"properties"`
}

// Edge represents a generic relationship (or edge) between two nodes in a Neo4j graph.
// It includes the relationship's unique ID, its type, its properties, and the unique
// ElementIds of the source and target nodes it connects.
type Edge struct {
	// ID is the unique internal identifier assigned by Neo4j to the relationship (ElementId).
	ID string `json:"id"`

	// Source is the ElementId of the node where the relationship starts.
	Source string `json:"source"`

	// Target is the ElementId of the node where the relationship ends.
	Target string `json:"target"`

	// Type is the relationship's type (e.g., "WROTE", "FOLLOWS").
	Type string `json:"type"`

	// Properties is a map containing the key-value properties of the relationship.
	Properties map[string]interface{} `json:"properties"`
}

// GraphResult is a top-level container for a generic graph query result.
// It is composed of a list of nodes and a list of edges, which is a standard
// format consumed by most frontend graph visualization libraries (e.g., D3.js, Cytoscape.js).
type GraphResult struct {
	// Nodes contains all the unique nodes retrieved by the query.
	Nodes []*GraphNode `json:"nodes"`

	// Edges contains all the unique relationships retrieved by the query.
	Edges []*Edge `json:"edges"`
}
