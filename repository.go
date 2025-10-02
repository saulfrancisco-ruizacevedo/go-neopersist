// Package neopersist provides a generic repository pattern for Neo4j,
// simplifying CRUD (Create, Read, Update, Delete) operations.
package neopersist

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

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

// FindAll retrieves all entities of type T from the database.
// It performs a `MATCH (n:Label) RETURN n` query. Use with caution on large datasets,
// as this can consume significant memory.
//
// Returns:
//
//	A slice of pointers to the found entities. Returns an empty slice if no entities are found.
func (r *Repository[T]) FindAll(ctx context.Context) ([]*T, error) {
	query, params, err := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label)).
		Return("n").
		Build()
	if err != nil {
		return nil, err
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		// An empty result set is not considered an error for FindAll.
		if errors.Is(err, ErrNotFound) {
			return []*T{}, nil
		}
		return nil, err
	}

	// Map all resulting records to a slice of entity structs.
	entities := make([]*T, len(eagerResult.Records))
	for i, record := range eagerResult.Records {
		nodeValue, _ := record.Get("n")
		node := nodeValue.(neo4j.Node)

		entity := new(T)
		if err := mapNodeToStruct(node, entity, r.meta); err != nil {
			return nil, err // Return on the first mapping error.
		}
		entities[i] = entity
	}

	return entities, nil
}

// FindByProperty retrieves all entities of type T that match a specific property-value pair.
// This is useful for querying on non-primary-key fields (e.g., finding users by email).
//
// Parameters:
//   - propName: The name of the property in the Neo4j node (e.g., "email").
//   - propValue: The value to match for the given property.
//
// Returns:
//
//	A slice of pointers to the found entities. Returns an empty slice if no entities match.
func (r *Repository[T]) FindByProperty(ctx context.Context, propName string, propValue interface{}) ([]*T, error) {
	// Safety check: ensure the property name is a valid, mapped property for the entity.
	isMappedProperty := false
	for _, p := range r.meta.Mappings {
		if p == propName {
			isMappedProperty = true
			break
		}
	}
	if !isMappedProperty {
		return nil, fmt.Errorf("property '%s' is not a mapped property for entity type %s", propName, r.meta.Label)
	}

	// Build the MATCH query with the specified property.
	props := map[string]interface{}{propName: propValue}
	query, params, err := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label).WithProperties(props)).
		Return("n").
		Build()
	if err != nil {
		return nil, err
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []*T{}, nil
		}
		return nil, err
	}

	// Map all resulting records to a slice of entity structs.
	entities := make([]*T, len(eagerResult.Records))
	for i, record := range eagerResult.Records {
		nodeValue, _ := record.Get("n")
		node := nodeValue.(neo4j.Node)

		entity := new(T)
		if err := mapNodeToStruct(node, entity, r.meta); err != nil {
			return nil, err
		}
		entities[i] = entity
	}

	return entities, nil
}

// Find executes a custom query defined by a gocypher.QueryBuilder and intelligently
// maps the results to a slice of entities. This powerful and flexible method can
// hydrate both full or partial structs based on the query's RETURN clause.
//
// The method inspects each result record and maps the returned data to the fields
// of the entity struct T.
//   - If a full neo4j.Node is returned (e.g., `RETURN u`), all struct fields are populated.
//   - If specific properties are returned (e.g., `RETURN u.name, u.email`), only the
//     corresponding struct fields will be populated, leaving the others as their zero value.
//
// Example for a full entity:
//
//	qb := gocypher.NewQueryBuilder().
//	    Match(gocypher.N("u", "User")).
//	    Where("u.name CONTAINS 'A'").
//	    Return("u") // Returns the full node
//	users, err := userRepo.Find(ctx, qb)
//
// Example for a partial entity:
//
//	qb := gocypher.NewQueryBuilder().
//	    Match(gocypher.N("u", "User")).
//	    Return("u.name", "u.email") // Returns only two properties
//	users, err := userRepo.Find(ctx, qb)
//
// Parameters:
//   - qb: A configured gocypher.QueryBuilder instance that defines the query.
//
// Returns:
//
//	A slice of pointers to the found entities, populated with the data returned by
//	the query. Returns an empty slice if no records are found.
func (r *Repository[T]) Find(ctx context.Context, qb *gocypher.QueryBuilder) ([]*T, error) {
	query, params, err := qb.Build()
	if err != nil {
		return nil, fmt.Errorf("could not build query: %w", err)
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []*T{}, nil
		}
		return nil, err
	}

	var entities []*T

	// 1. Iterate over each record (row) returned by Neo4j.
	for _, record := range eagerResult.Records {
		entity := new(T)
		val := reflect.ValueOf(entity).Elem()

		// Optimization: Check if a full node is present in the result. If so, map it directly.
		// This is a common case (e.g., RETURN n) and is more efficient.
		var mappedFromNode bool
		for _, value := range record.Values {
			if node, ok := value.(neo4j.Node); ok {
				if err := mapNodeToStruct(node, entity, r.meta); err != nil {
					return nil, err
				}
				mappedFromNode = true
				break
			}
		}

		// 2. If the result did not contain a full node, hydrate the struct property by property.
		// This handles partial projections (e.g., RETURN u.name, u.email).
		if !mappedFromNode {
			// Iterate over the Go struct's mapped fields.
			for goFieldName, neo4jPropName := range r.meta.Mappings {
				field := val.FieldByName(goFieldName)

				// 3. Find a key in the result record that matches the struct's property name.
				// This works for direct aliases (`RETURN u.name AS name`) and for property projections (`RETURN u.name`).
				var foundValue any
				var found bool
				for _, key := range record.Keys {
					if key == neo4jPropName || strings.HasSuffix(key, "."+neo4jPropName) {
						foundValue, found = record.Get(key)
						break
					}
				}

				// 4. If a matching value was found, set it on the corresponding struct field.
				if found && field.IsValid() && field.CanSet() {
					if foundValue != nil {
						field.Set(reflect.ValueOf(foundValue))
					}
				}
			}
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// FindOne executes a query expected to return a single entity.
// It uses the same intelligent mapping as the Find method but validates the result set.
//
// This method is ideal for finding entities by unique properties other than the primary key
// (e.g., finding a user by their unique email address).
//
// Returns:
//   - A pointer to the found entity if exactly one record is returned.
//   - An ErrNotFound error if the query returns zero records.
//   - An error if the query returns more than one record, indicating a data consistency issue.
//   - Any other error encountered during query execution or mapping.
func (r *Repository[T]) FindOne(ctx context.Context, qb *gocypher.QueryBuilder) (*T, error) {
	query, params, err := qb.Build()
	if err != nil {
		return nil, fmt.Errorf("could not build query: %w", err)
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		return nil, err // This will propagate ErrNotFound from the runner if applicable.
	}

	// --- Result Validation ---
	if len(eagerResult.Records) == 0 {
		return nil, ErrNotFound
	}
	if len(eagerResult.Records) > 1 {
		return nil, fmt.Errorf("expected 1 record but found %d", len(eagerResult.Records))
	}

	// --- Mapping Logic (reused from Find) ---
	record := eagerResult.Records[0]
	entity := new(T)
	val := reflect.ValueOf(entity).Elem()

	var mappedFromNode bool
	for _, value := range record.Values {
		if node, ok := value.(neo4j.Node); ok {
			if err := mapNodeToStruct(node, entity, r.meta); err != nil {
				return nil, err
			}
			mappedFromNode = true
			break
		}
	}

	if !mappedFromNode {
		for goFieldName, neo4jPropName := range r.meta.Mappings {
			field := val.FieldByName(goFieldName)
			var foundValue any
			var found bool
			for _, key := range record.Keys {
				if key == neo4jPropName || strings.HasSuffix(key, "."+neo4jPropName) {
					foundValue, found = record.Get(key)
					break
				}
			}
			if found && field.IsValid() && field.CanSet() {
				if foundValue != nil {
					field.Set(reflect.ValueOf(foundValue))
				}
			}
		}
	}

	return entity, nil
}

// FindFirst executes a query and returns only the first entity found, ignoring any
// additional results. It is useful for queries with ordering and limits where you
// only care about the top result (e.g., finding the most recently created post).
//
// Unlike FindOne, this method will not return an error if the query finds more
// than one record; it will simply map and return the first one in the result set.
//
// Returns:
//   - A pointer to the first found entity.
//   - An ErrNotFound error if the query returns zero records.
//   - Any other error encountered during query execution or mapping.
func (r *Repository[T]) FindFirst(ctx context.Context, qb *gocypher.QueryBuilder) (*T, error) {
	query, params, err := qb.Build()
	if err != nil {
		return nil, fmt.Errorf("could not build query: %w", err)
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		return nil, err // This will propagate ErrNotFound from the runner if applicable.
	}

	// --- Result Validation ---
	if len(eagerResult.Records) == 0 {
		return nil, ErrNotFound
	}
	// Note: We do NOT check for len > 1. We intentionally take the first result.

	// --- Mapping Logic (same as FindOne) ---
	record := eagerResult.Records[0]
	entity := new(T)
	val := reflect.ValueOf(entity).Elem()

	var mappedFromNode bool
	for _, value := range record.Values {
		if node, ok := value.(neo4j.Node); ok {
			if err := mapNodeToStruct(node, entity, r.meta); err != nil {
				return nil, err
			}
			mappedFromNode = true
			break
		}
	}

	if !mappedFromNode {
		for goFieldName, neo4jPropName := range r.meta.Mappings {
			field := val.FieldByName(goFieldName)
			var foundValue any
			var found bool
			for _, key := range record.Keys {
				if key == neo4jPropName || strings.HasSuffix(key, "."+neo4jPropName) {
					foundValue, found = record.Get(key)
					break
				}
			}
			if found && field.IsValid() && field.CanSet() {
				if foundValue != nil {
					field.Set(reflect.ValueOf(foundValue))
				}
			}
		}
	}

	return entity, nil
}

// Count returns the total number of entities of type T in the database.
// It performs a `MATCH (n:Label) RETURN count(n)` query.
func (r *Repository[T]) Count(ctx context.Context) (int64, error) {
	qb := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label)).
		Return("count(n) AS count")

	query, params, err := qb.Build()
	if err != nil {
		return 0, fmt.Errorf("could not build count query: %w", err)
	}

	// We use the raw runner because we expect a number, not an entity.
	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		return 0, err
	}

	if len(eagerResult.Records) == 0 {
		// This case is unlikely for a count query but is a safe check.
		return 0, nil
	}

	record := eagerResult.Records[0]
	countValue, ok := record.Get("count")
	if !ok {
		return 0, fmt.Errorf("count value not found in query result")
	}

	return countValue.(int64), nil
}

// CountByProperty returns the number of entities of type T that match a specific
// property-value pair.
//
// Parameters:
//   - propName: The name of the property in the Neo4j node.
//   - propValue: The value to match for the given property.
func (r *Repository[T]) CountByProperty(ctx context.Context, propName string, propValue interface{}) (int64, error) {
	// ... (puedes añadir la misma validación de propiedad que en FindByProperty) ...

	props := map[string]interface{}{propName: propValue}
	qb := gocypher.NewQueryBuilder().
		Match(gocypher.N("n", r.meta.Label).WithProperties(props)).
		Return("count(n) AS count")

	query, params, err := qb.Build()
	if err != nil {
		return 0, fmt.Errorf("could not build count query: %w", err)
	}

	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		return 0, err
	}

	if len(eagerResult.Records) == 0 {
		return 0, nil
	}

	record := eagerResult.Records[0]
	countValue, ok := record.Get("count")
	if !ok {
		return 0, fmt.Errorf("count value not found in query result")
	}

	return countValue.(int64), nil
}

// CountWithQuery executes a custom query and returns the resulting count.
// This method is the flexible counterpart to the simple Count() and CountByProperty() methods.
// The provided QueryBuilder is expected to define the MATCH and WHERE logic, and crucially,
// it MUST include a RETURN clause that returns a single numerical value with the
// alias "count".
//
// Example:
//
//	qb := gocypher.NewQueryBuilder().
//	    Match(gocypher.N("u", "User")).
//	    Where("u.age > 30").
//	    Return("count(u) AS count") // The "AS count" is required.
//	total, err := userRepo.CountWithQuery(ctx, qb)
func (r *Repository[T]) CountWithQuery(ctx context.Context, qb *gocypher.QueryBuilder) (int64, error) {
	query, params, err := qb.Build()
	if err != nil {
		return 0, fmt.Errorf("could not build count query: %w", err)
	}

	// We use the raw runner because we expect a number, not an entity.
	eagerResult, err := r.runner.Run(ctx, query, params)
	if err != nil {
		// If the query returns no rows (e.g., MATCH fails), the count is 0.
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}

	if len(eagerResult.Records) == 0 {
		return 0, nil // A successful query with no records means a count of 0.
	}

	// The query should return a single record with a single "count" value.
	record := eagerResult.Records[0]
	countValue, ok := record.Get("count")
	if !ok {
		return 0, fmt.Errorf("a 'count' value was not returned by the query; ensure your query includes 'RETURN count(...) AS count'")
	}

	return countValue.(int64), nil
}
