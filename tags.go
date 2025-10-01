package neopersist

import (
	"fmt"
	"reflect"
	"strings"
)

// entityMetadata holds the parsed `crud` tag information for a specific struct type.
// This metadata is cached by the PersistenceManager to avoid costly reflection on every operation.
type entityMetadata struct {
	// Label is the graph node label, defaulting to the struct's name.
	Label string
	// PKField is the name of the struct field marked as the primary key.
	PKField string
	// PKProp is the property name of the primary key in the database.
	PKProp string
	// Mappings maps struct field names to their corresponding database property names.
	Mappings map[string]string
}

// parseTagsFromType is the core non-generic function that inspects a reflect.Type
// and extracts persistence metadata from `crud` struct tags. It serves as the reusable
// heart of the tag parsing logic, usable in both generic and dynamic contexts.
func parseTagsFromType(typ reflect.Type) (*entityMetadata, error) {
	// If the type is a pointer, get the underlying element's type.
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type %s is not a struct", typ.Name())
	}

	meta := &entityMetadata{
		Label:    typ.Name(),
		Mappings: make(map[string]string),
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("crud")

		// Skip fields that are not part of the persistence mapping.
		if tag == "" {
			continue
		}

		parts := strings.Split(tag, ",")
		isPk := false
		propName := ""

		for _, part := range parts {
			if part == "pk" {
				isPk = true
			}
			if strings.HasPrefix(part, "property:") {
				propName = strings.TrimPrefix(part, "property:")
			}
		}

		if propName == "" {
			return nil, fmt.Errorf("field %s is missing 'property' tag component", field.Name)
		}

		if isPk {
			meta.PKField = field.Name
			meta.PKProp = propName
		}
		meta.Mappings[field.Name] = propName
	}

	if meta.PKField == "" {
		return nil, fmt.Errorf("no primary key ('pk') tag defined for struct %s", typ.Name())
	}

	return meta, nil
}

// parseTags is a generic convenience wrapper around parseTagsFromType.
// It allows getting metadata from a compile-time type T instead of a runtime reflect.Type,
// which is useful for the generic Repository.
func parseTags[T any]() (*entityMetadata, error) {
	var instance T
	typ := reflect.TypeOf(instance)
	return parseTagsFromType(typ)
}
