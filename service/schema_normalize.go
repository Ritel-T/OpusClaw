package service

import "strings"

func NormalizeSchemaTypes(params any) any {
	return normalizeSchemaTypesWithDepth(params, 0, 64)
}

func normalizeSchemaTypesWithDepth(params any, depth int, maxDepth int) any {
	if params == nil {
		return nil
	}
	if depth >= maxDepth {
		return normalizeSchemaTypesShallow(params)
	}

	switch v := params.(type) {
	case map[string]any:
		cleanedMap := make(map[string]any, len(v))
		for key, value := range v {
			cleanedMap[key] = value
		}

		collapseSchemaCombinators(cleanedMap)
		normalizeSchemaTypeAndNullable(cleanedMap)

		if props, ok := cleanedMap["properties"].(map[string]any); ok && props != nil {
			cleanedProps := make(map[string]any, len(props))
			for propName, propValue := range props {
				cleanedProps[propName] = normalizeSchemaTypesWithDepth(propValue, depth+1, maxDepth)
			}
			cleanedMap["properties"] = cleanedProps
		}

		if items, ok := cleanedMap["items"].(map[string]any); ok && items != nil {
			cleanedMap["items"] = normalizeSchemaTypesWithDepth(items, depth+1, maxDepth)
		}
		if itemsArray, ok := cleanedMap["items"].([]any); ok && len(itemsArray) > 0 {
			cleanedMap["items"] = normalizeSchemaTypesWithDepth(itemsArray[0], depth+1, maxDepth)
		}

		return cleanedMap
	case []any:
		cleanedArray := make([]any, len(v))
		for i, item := range v {
			cleanedArray[i] = normalizeSchemaTypesWithDepth(item, depth+1, maxDepth)
		}
		return cleanedArray
	default:
		return params
	}
}

func normalizeSchemaTypesShallow(params any) any {
	switch v := params.(type) {
	case map[string]any:
		cleanedMap := make(map[string]any, len(v))
		for key, value := range v {
			cleanedMap[key] = value
		}
		collapseSchemaCombinators(cleanedMap)
		normalizeSchemaTypeAndNullable(cleanedMap)
		delete(cleanedMap, "properties")
		delete(cleanedMap, "items")
		delete(cleanedMap, "anyOf")
		delete(cleanedMap, "oneOf")
		delete(cleanedMap, "allOf")
		return cleanedMap
	case []any:
		return []any{}
	default:
		return params
	}
}

func collapseSchemaCombinators(schema map[string]any) {
	if schema == nil {
		return
	}
	if _, hasType := schema["type"]; !hasType {
		if inferredType, nullable := inferSchemaTypeFromCombinators(schema); inferredType != "" {
			schema["type"] = inferredType
			if nullable {
				schema["nullable"] = true
			}
		}
	}
	delete(schema, "anyOf")
	delete(schema, "oneOf")
	delete(schema, "allOf")
}

func normalizeSchemaTypeAndNullable(schema map[string]any) {
	rawType, ok := schema["type"]
	if !ok || rawType == nil {
		if _, hasProperties := schema["properties"]; hasProperties {
			schema["type"] = "object"
			return
		}
		if _, hasItems := schema["items"]; hasItems {
			schema["type"] = "array"
			return
		}
		if _, hasEnum := schema["enum"]; hasEnum {
			schema["type"] = "string"
			return
		}
		return
	}

	switch t := rawType.(type) {
	case string:
		normalized, isNull := normalizeSchemaPrimitiveType(t)
		if isNull {
			schema["nullable"] = true
			delete(schema, "type")
			return
		}
		schema["type"] = normalized
	case []any:
		nullable := false
		var chosen string
		for _, item := range t {
			if s, ok := item.(string); ok {
				normalized, isNull := normalizeSchemaPrimitiveType(s)
				if isNull {
					nullable = true
					continue
				}
				if chosen == "" {
					chosen = normalized
				}
			}
		}
		if nullable {
			schema["nullable"] = true
		}
		if chosen != "" {
			schema["type"] = chosen
		} else {
			delete(schema, "type")
		}
	}
}

func inferSchemaTypeFromCombinators(schema map[string]any) (string, bool) {
	for _, field := range []string{"anyOf", "oneOf", "allOf"} {
		raw, ok := schema[field]
		if !ok {
			continue
		}
		variants, ok := raw.([]any)
		if !ok || len(variants) == 0 {
			continue
		}
		nullable := false
		chosen := ""
		for _, variant := range variants {
			variantMap, ok := variant.(map[string]any)
			if !ok {
				continue
			}
			variantType, variantNullable := inferSchemaType(variantMap)
			if variantNullable {
				nullable = true
			}
			if variantType != "" && chosen == "" {
				chosen = variantType
			}
		}
		return chosen, nullable
	}
	return "", false
}

func inferSchemaType(schema map[string]any) (string, bool) {
	if schema == nil {
		return "", false
	}
	if rawType, ok := schema["type"]; ok && rawType != nil {
		switch t := rawType.(type) {
		case string:
			normalized, isNull := normalizeSchemaPrimitiveType(t)
			return normalized, isNull
		case []any:
			nullable := false
			for _, item := range t {
				if s, ok := item.(string); ok {
					normalized, isNull := normalizeSchemaPrimitiveType(s)
					if isNull {
						nullable = true
						continue
					}
					if normalized != "" {
						return normalized, nullable
					}
				}
			}
			return "", nullable
		}
	}
	if _, hasProperties := schema["properties"]; hasProperties {
		return "object", false
	}
	if _, hasItems := schema["items"]; hasItems {
		return "array", false
	}
	if _, hasEnum := schema["enum"]; hasEnum {
		return "string", false
	}
	return inferSchemaTypeFromCombinators(schema)
}

func normalizeSchemaPrimitiveType(t string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "object":
		return "object", false
	case "array":
		return "array", false
	case "string":
		return "string", false
	case "integer":
		return "integer", false
	case "number":
		return "number", false
	case "boolean":
		return "boolean", false
	case "null":
		return "", true
	default:
		return t, false
	}
}
