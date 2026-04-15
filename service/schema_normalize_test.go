package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSchemaTypesLowercasesAndInfersTypes(t *testing.T) {
	t.Parallel()

	params := map[string]any{
		"properties": map[string]any{
			"path":  map[string]any{"type": "STRING"},
			"count": map[string]any{"type": []any{"INTEGER", "null"}},
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

	cleaned, ok := NormalizeSchemaTypes(params).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "object", cleaned["type"])

	properties := cleaned["properties"].(map[string]any)
	require.Equal(t, "string", properties["path"].(map[string]any)["type"])

	count := properties["count"].(map[string]any)
	require.Equal(t, "integer", count["type"])
	require.Equal(t, true, count["nullable"])

	edits := properties["edits"].(map[string]any)
	require.Equal(t, "object", edits["type"])
	lines := edits["properties"].(map[string]any)["lines"].(map[string]any)
	require.Equal(t, "array", lines["type"])
	require.Equal(t, "string", lines["items"].(map[string]any)["type"])
}

func TestNormalizeSchemaTypesCollapsesCombinators(t *testing.T) {
	t.Parallel()

	params := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "STRING"},
					map[string]any{"type": "null"},
				},
			},
		},
	}

	cleaned := NormalizeSchemaTypes(params).(map[string]any)
	pathSchema := cleaned["properties"].(map[string]any)["path"].(map[string]any)
	require.Equal(t, "string", pathSchema["type"])
	require.Equal(t, true, pathSchema["nullable"])
	_, hasAnyOf := pathSchema["anyOf"]
	require.False(t, hasAnyOf)
}
