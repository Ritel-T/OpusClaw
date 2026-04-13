package gemini

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanFunctionParametersNormalizesTypesToLowercaseJSONSchema(t *testing.T) {
	t.Parallel()

	params := map[string]any{
		"type": "OBJECT",
		"properties": map[string]any{
			"path": map[string]any{
				"type": "STRING",
			},
			"threshold": map[string]any{
				"type": "NUMBER",
			},
			"retry": map[string]any{
				"type": []any{"INTEGER", "null"},
			},
		},
	}

	cleaned, ok := cleanFunctionParameters(params).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", cleaned["type"])

	properties, ok := cleaned["properties"].(map[string]any)
	require.True(t, ok)

	pathSchema, ok := properties["path"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "string", pathSchema["type"])

	thresholdSchema, ok := properties["threshold"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "number", thresholdSchema["type"])

	retrySchema, ok := properties["retry"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "integer", retrySchema["type"])
	require.Equal(t, true, retrySchema["nullable"])
}

func TestCleanFunctionParametersInfersMissingTypes(t *testing.T) {
	t.Parallel()

	params := map[string]any{
		"properties": map[string]any{
			"edits": map[string]any{
				"properties": map[string]any{
					"lines": map[string]any{
						"items": map[string]any{
							"enum": []any{"before", "after"},
						},
					},
				},
			},
		},
	}

	cleaned, ok := cleanFunctionParameters(params).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", cleaned["type"])

	properties := cleaned["properties"].(map[string]any)
	edits := properties["edits"].(map[string]any)
	require.Equal(t, "object", edits["type"])

	editsProperties := edits["properties"].(map[string]any)
	lines := editsProperties["lines"].(map[string]any)
	require.Equal(t, "array", lines["type"])

	items := lines["items"].(map[string]any)
	require.Equal(t, "string", items["type"])
	require.Equal(t, []any{"before", "after"}, items["enum"])
}

func TestCleanFunctionParametersCollapsesAnyOfToStableType(t *testing.T) {
	t.Parallel()

	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "null"},
				},
			},
			"count": map[string]any{
				"oneOf": []any{
					map[string]any{"type": "integer"},
					map[string]any{"type": "number"},
				},
			},
		},
	}

	cleaned := cleanFunctionParameters(params).(map[string]any)
	properties := cleaned["properties"].(map[string]any)

	pathSchema := properties["path"].(map[string]any)
	require.Equal(t, "string", pathSchema["type"])
	require.Equal(t, true, pathSchema["nullable"])
	_, hasAnyOf := pathSchema["anyOf"]
	require.False(t, hasAnyOf)

	countSchema := properties["count"].(map[string]any)
	require.Equal(t, "integer", countSchema["type"])
	_, hasOneOf := countSchema["oneOf"]
	require.False(t, hasOneOf)
}
