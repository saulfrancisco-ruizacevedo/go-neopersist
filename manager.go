package neopersist

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/saulfrancisco-ruizacevedo/go-neopersist/examples/models"
	"github.com/saulfrancisco-ruizacevedo/gocypher"
)

// PersistenceManager is the central orchestrator for the persistence layer.
// It manages the database connection and provides access to repositories and complex,
// cross-entity operations like creating relationships.
type PersistenceManager struct {
	runner DBRunner
	// metaCache stores parsed entityMetadata to avoid costly reflection on every call.
	metaCache sync.Map
}

// NewPersistenceManager creates a new instance of the PersistenceManager.
func NewPersistenceManager(runner DBRunner) *PersistenceManager {
	return &PersistenceManager{runner: runner}
}

// RepositoryFor is a generic function that creates and returns a repository
// for a specific struct type T, managed by the given PersistenceManager.
func RepositoryFor[T any](pm *PersistenceManager) (*Repository[T], error) {
	return NewRepository[T](pm.runner)
}

// CreateRelation creates a directed relationship between two existing entities in the database.
// It uses reflection to find the entities' primary keys and labels to build the query.
func (pm *PersistenceManager) CreateRelation(ctx context.Context, fromEntity any, toEntity any, relType string, relProps map[string]interface{}) error {
	fromMeta, fromPKVal, err := pm.getEntityMetaAndPK(fromEntity)
	if err != nil {
		return err
	}
	toMeta, toPKVal, err := pm.getEntityMetaAndPK(toEntity)
	if err != nil {
		return err
	}

	qb := gocypher.NewQueryBuilder().
		Match(gocypher.N("a", fromMeta.Label).WithProperties(map[string]interface{}{fromMeta.PKProp: fromPKVal})).
		Match(gocypher.N("b", toMeta.Label).WithProperties(map[string]interface{}{toMeta.PKProp: toPKVal})).
		Create(
			gocypher.N("a", ""), // Reference the 'a' alias without its label
			gocypher.R("r", relType).To().WithProperties(relProps),
			gocypher.N("b", ""), // Reference the 'b' alias without its label
		)

	query, params, err := qb.Build()
	if err != nil {
		return err
	}

	_, err = pm.runner.Run(ctx, query, params)
	if err != nil {
		return err
	}
	return nil
}

// getEntityMetaAndPK is an internal helper that retrieves an entity's metadata and primary key value.
// It uses a cache to optimize performance by avoiding repeated reflection.
func (pm *PersistenceManager) getEntityMetaAndPK(entity any) (*entityMetadata, any, error) {
	val := reflect.ValueOf(entity)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return nil, nil, fmt.Errorf("entity must be a non-nil pointer")
	}

	typ := val.Elem().Type()

	// First, attempt to load metadata from the cache for performance.
	if cached, ok := pm.metaCache.Load(typ); ok {
		meta := cached.(*entityMetadata)
		pkValue := val.Elem().FieldByName(meta.PKField).Interface()
		return meta, pkValue, nil
	}

	// If not found in cache, parse the tags using reflection.
	meta, err := parseTagsFromType(typ)
	if err != nil {
		return nil, nil, err
	}
	// Store the newly parsed metadata in the cache for future use.
	pm.metaCache.Store(typ, meta)

	pkValue := val.Elem().FieldByName(meta.PKField).Interface()
	return meta, pkValue, nil
}

// FindGraph executes a graph query defined by a gocypher.QueryBuilder and maps the result
// into a generic graph structure composed of nodes and edges.
//
// This method is domain-agnostic; it does not need to know about specific Go structs
// like User or Post. Its primary role is to translate the raw graph elements returned by
// a Cypher query into a clean, serializable format suitable for frontends or other services.
//
// The caller is responsible for constructing a valid query via the QueryBuilder, including
// a RETURN clause that specifies which nodes and relationships should be included in the
// final graph. For example, `RETURN u, r, p`.
//
// The function intelligently de-duplicates nodes and relationships, ensuring that even if a
// graph element is returned in multiple rows of the result set, it will only appear
// once in the final GraphResult.
//
// Parameters:
//   - ctx: The context for the query execution.
//   - qb: A pointer to a configured gocypher.QueryBuilder instance that defines the graph to retrieve.
//
// Returns:
//   - A pointer to a models.GraphResult containing the de-duplicated nodes and edges from the query.
//   - An ErrNotFound error if the query executes successfully but returns zero records.
//   - Any other error encountered during query building or execution.
func (pm *PersistenceManager) FindGraph(ctx context.Context, qb *gocypher.QueryBuilder) (*models.GraphResult, error) {
	// 1. Build and execute the query provided by the client.
	query, params, err := qb.Build()
	if err != nil {
		return nil, fmt.Errorf("could not build query: %w", err)
	}

	eagerResult, err := pm.runner.Run(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(eagerResult.Records) == 0 {
		return nil, ErrNotFound
	}

	// 2. Prepare the result structure and maps for de-duplication.
	graph := &models.GraphResult{
		Nodes: make([]*models.GraphNode, 0),
		Edges: make([]*models.Edge, 0),
	}
	seenNodeIDs := make(map[string]bool)
	seenEdgeIDs := make(map[string]bool)

	// 3. Iterate over the records and their values to populate the graph.
	for _, record := range eagerResult.Records {
		// Iterate over each value in the result row (e.g., the returned u, r, p).
		for _, value := range record.Values {

			// Use a type switch to process nodes and relationships from the result.
			switch v := value.(type) {
			case neo4j.Node:
				// If this node has not been seen yet, process and add it.
				if !seenNodeIDs[v.ElementId] {
					graph.Nodes = append(graph.Nodes, &models.GraphNode{
						ID:         v.ElementId,
						Labels:     v.Labels,
						Properties: v.Props,
					})
					seenNodeIDs[v.ElementId] = true
				}

			case neo4j.Relationship:
				// If this relationship has not been seen yet, process and add it.
				if !seenEdgeIDs[v.ElementId] {
					graph.Edges = append(graph.Edges, &models.Edge{
						ID:         v.ElementId,
						Source:     v.StartElementId,
						Target:     v.EndElementId,
						Type:       v.Type,
						Properties: v.Props,
					})
					seenEdgeIDs[v.ElementId] = true
				}
			}
		}
	}

	return graph, nil
}
