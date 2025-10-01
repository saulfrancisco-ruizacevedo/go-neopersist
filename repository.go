// Package neopersist provides a generic repository pattern for Neo4j,
// simplifying CRUD (Create, Read, Update, Delete) operations.
package neopersist

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/saulfrancisco-ruizacevedo/gocypher"
)

// ErrNotFound is a sentinel error returned by Find operations when no record
// matching the criteria is found in the database.
var ErrNotFound = errors.New("record not found")

// Repository provides a generic abstraction for CRUD operations for a specific
// entity type T. It relies on struct tags to map struct fields to node properties.
type Repository[T any] struct {
	runner DBRunner
	meta   *entityMetadata
}

// NewRepository creates a new generic repository for the type T.
// It parses the struct tags of T to understand its mapping to a Neo4j node.
//
// Parameters:
//   - runner: An instance of DBRunner, used to execute all Cypher queries.
//
// Returns:
//
//	A new Repository instance or an error if the struct tags are invalid.
func NewRepository[T any](runner DBRunner) (*Repository[T], error) {
	meta, err := parseTags[T]()
	if err != nil {
		return nil, err
	}
	return &Repository[T]{
		runner: runner,
		meta:   meta,
	}, nil
}

// Save creates a new node or updates an existing one.
// It uses a MERGE query based on the struct's primary key (`pk` tag).
// All other tagged fields are set on the node.
//
// Parameters:
//   - ctx: The context for the query execution.
//   - entity: A pointer to the struct instance to be saved.
//
// Returns:
//
//	An error if the query building or execution fails.
func (r *Repository[T]) Save(ctx context.Context, entity *T) error {
	val := reflect.ValueOf(entity).Elem()
	pkValue := val.FieldByName(r.meta.PKField).Interface()
	mergeProps := map[string]interface{}{r.meta.PKProp: pkValue}

	setProps := make(map[string]interface{})
	for fieldName, propName := range r.meta.Mappings {
		if fieldName != r.meta.PKField {
			// The property is prefixed with 'n.' for the SET clause.
			setProps["n."+propName] = val.FieldByName(fieldName).Interface()
		}
	}

	qb := gocypher.NewQueryBuilder().
		Merge(gocypher.N("n", r.meta.Label).WithProperties(mergeProps)).
		Set(setProps).
		Return("n")

	query, params, err := qb.Build()
	if err != nil {
		return err
	}
	_, err = r.runner.Run(ctx, query, params)
	return err
}

// FindByID retrieves a single entity from the database by its primary key.
//
// Parameters:
//   - ctx: The context for the query execution.
//   - id: The primary key value of the entity to find.
//
// Returns:
//
//	A pointer to the found entity, ErrNotFound if no record is found, or another
//	error if the query or mapping fails.
func (r *Repository[T]) FindByID(ctx context.Context, id interface{}) (*T, error) {
	// 1. Build the query using gocypher.
	props := map[string]interface{}{r.meta.PKProp: id}
	query, params, err := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label).WithProperties(props)).
		Return("n").
		Build()
	if err != nil {
		return nil, err
	}

	// 2. Execute the query using the runner.
	// The result is an EagerResult, which contains a slice of all records.
	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		return nil, err
	}

	// 3. Process the result records.
	if len(eagerResult.Records) == 0 {
		return nil, ErrNotFound
	}
	if len(eagerResult.Records) > 1 {
		// This indicates a data integrity issue, as a primary key lookup should be unique.
		return nil, fmt.Errorf("expected 1 record but found %d", len(eagerResult.Records))
	}

	record := eagerResult.Records[0]
	nodeValue, ok := record.Get("n")
	if !ok {
		return nil, fmt.Errorf("could not find return value 'n' in query result")
	}

	node, ok := nodeValue.(neo4j.Node)
	if !ok {
		return nil, fmt.Errorf("return value 'n' is not a node")
	}

	// 4. Map the node properties to a new struct instance.
	entity := new(T)
	if err := mapNodeToStruct(node, entity, r.meta); err != nil {
		return nil, err
	}

	return entity, nil
}

// Delete removes a node from the database by its primary key.
// It uses a DETACH DELETE query to also remove any relationships connected to the node.
//
// Parameters:
//   - ctx: The context for the query execution.
//   - id: The primary key value of the entity to delete.
//
// Returns:
//
//	An error if the query building or execution fails.
func (r *Repository[T]) Delete(ctx context.Context, id interface{}) error {
	props := map[string]interface{}{r.meta.PKProp: id}
	query, params, err := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label).WithProperties(props)).
		DetachDelete("n").
		Build()
	if err != nil {
		return err
	}
	_, err = r.runner.Run(ctx, query, params)
	return err
}

// mapNodeToStruct is an internal helper function that populates a struct's fields
// from a neo4j.Node's properties, based on the parsed metadata.
func mapNodeToStruct(node neo4j.Node, entity any, meta *entityMetadata) error {
	val := reflect.ValueOf(entity).Elem()

	for fieldName, propName := range meta.Mappings {
		field := val.FieldByName(fieldName)
		if !field.IsValid() || !field.CanSet() {
			continue // Skip if the struct field cannot be set.
		}

		propValue, ok := node.Props[propName]
		if !ok {
			continue // Skip if the property does not exist on the node.
		}

		// Set the struct field's value.
		field.Set(reflect.ValueOf(propValue))
	}
	return nil
}
