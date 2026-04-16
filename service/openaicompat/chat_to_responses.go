package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

type pendingToolOutput struct {
	callID string
	name   string
	output any
	index  int
}

type pendingFunctionCall struct {
	callID    string
	name      string
	arguments any
	index     int
}

func normalizeResponsesSchemaTypes(params any) any {
	return normalizeResponsesSchemaTypesWithDepth(params, 0, 64)
}

func normalizeResponsesSchemaTypesWithDepth(params any, depth int, maxDepth int) any {
	if params == nil {
		return nil
	}
	if depth >= maxDepth {
		return normalizeResponsesSchemaTypesShallow(params)
	}

	switch v := params.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, value := range v {
			cleaned[key] = value
		}
		collapseResponsesSchemaCombinators(cleaned)
		normalizeResponsesSchemaTypeAndNullable(cleaned)
		if props, ok := cleaned["properties"].(map[string]any); ok && props != nil {
			cleanedProps := make(map[string]any, len(props))
			for key, value := range props {
				cleanedProps[key] = normalizeResponsesSchemaTypesWithDepth(value, depth+1, maxDepth)
			}
			cleaned["properties"] = cleanedProps
		}
		if items, ok := cleaned["items"].(map[string]any); ok && items != nil {
			cleaned["items"] = normalizeResponsesSchemaTypesWithDepth(items, depth+1, maxDepth)
		}
		if itemsArray, ok := cleaned["items"].([]any); ok && len(itemsArray) > 0 {
			cleaned["items"] = normalizeResponsesSchemaTypesWithDepth(itemsArray[0], depth+1, maxDepth)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(v))
		for i, item := range v {
			cleaned[i] = normalizeResponsesSchemaTypesWithDepth(item, depth+1, maxDepth)
		}
		return cleaned
	default:
		return params
	}
}

func normalizeResponsesSchemaTypesShallow(params any) any {
	switch v := params.(type) {
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, value := range v {
			cleaned[key] = value
		}
		collapseResponsesSchemaCombinators(cleaned)
		normalizeResponsesSchemaTypeAndNullable(cleaned)
		delete(cleaned, "properties")
		delete(cleaned, "items")
		delete(cleaned, "anyOf")
		delete(cleaned, "oneOf")
		delete(cleaned, "allOf")
		return cleaned
	case []any:
		return []any{}
	default:
		return params
	}
}

func collapseResponsesSchemaCombinators(schema map[string]any) {
	if schema == nil {
		return
	}
	if _, hasType := schema["type"]; !hasType {
		if inferredType, nullable := inferResponsesSchemaTypeFromCombinators(schema); inferredType != "" {
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

func normalizeResponsesSchemaTypeAndNullable(schema map[string]any) {
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
		normalized, isNull := normalizeResponsesSchemaPrimitiveType(t)
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
				normalized, isNull := normalizeResponsesSchemaPrimitiveType(s)
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

func inferResponsesSchemaTypeFromCombinators(schema map[string]any) (string, bool) {
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
			variantType, variantNullable := inferResponsesSchemaType(variantMap)
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

func inferResponsesSchemaType(schema map[string]any) (string, bool) {
	if schema == nil {
		return "", false
	}
	if rawType, ok := schema["type"]; ok && rawType != nil {
		switch t := rawType.(type) {
		case string:
			normalized, isNull := normalizeResponsesSchemaPrimitiveType(t)
			return normalized, isNull
		case []any:
			nullable := false
			for _, item := range t {
				if s, ok := item.(string); ok {
					normalized, isNull := normalizeResponsesSchemaPrimitiveType(s)
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
	return inferResponsesSchemaTypeFromCombinators(schema)
}

func normalizeResponsesSchemaPrimitiveType(t string) (string, bool) {
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

func normalizeChatImageURLToString(v any) any {
	switch vv := v.(type) {
	case string:
		return vv
	case map[string]any:
		if url := common.Interface2String(vv["url"]); url != "" {
			return url
		}
		return v
	case dto.MessageImageUrl:
		if vv.Url != "" {
			return vv.Url
		}
		return v
	case *dto.MessageImageUrl:
		if vv != nil && vv.Url != "" {
			return vv.Url
		}
		return v
	default:
		return v
	}
}

func convertChatResponseFormatToResponsesText(reqFormat *dto.ResponseFormat) json.RawMessage {
	if reqFormat == nil || strings.TrimSpace(reqFormat.Type) == "" {
		return nil
	}

	format := map[string]any{
		"type": reqFormat.Type,
	}

	if reqFormat.Type == "json_schema" && len(reqFormat.JsonSchema) > 0 {
		var chatSchema map[string]any
		if err := common.Unmarshal(reqFormat.JsonSchema, &chatSchema); err == nil {
			for key, value := range chatSchema {
				if key == "type" {
					continue
				}
				format[key] = value
			}

			if nested, ok := format["json_schema"].(map[string]any); ok {
				for key, value := range nested {
					if _, exists := format[key]; !exists {
						format[key] = value
					}
				}
				delete(format, "json_schema")
			}
		} else {
			format["json_schema"] = reqFormat.JsonSchema
		}
	}

	textRaw, _ := common.Marshal(map[string]any{
		"format": format,
	})
	return textRaw
}

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if lo.FromPtrOr(req.N, 1) > 1 {
		return nil, fmt.Errorf("n>1 is not supported in responses compatibility mode")
	}

	var instructionsParts []string
	inputItems := make([]map[string]any, 0, len(req.Messages))
	toolCallNamesByID := make(map[string]string)
	pendingToolOutputs := make([]pendingToolOutput, 0)
	pendingFunctionCalls := make([]pendingFunctionCall, 0)
	functionCallIndex := 0
	toolOutputIndex := 0

	for _, msg := range req.Messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			continue
		}

		if role == "tool" || role == "function" {
			callID := strings.TrimSpace(msg.ToolCallId)
			name := ""
			if msg.Name != nil {
				name = strings.TrimSpace(*msg.Name)
			}

			var output any
			if msg.Content == nil {
				output = ""
			} else if msg.IsStringContent() {
				output = msg.StringContent()
			} else {
				if b, err := common.Marshal(msg.Content); err == nil {
					output = string(b)
				} else {
					output = fmt.Sprintf("%v", msg.Content)
				}
			}

			if callID == "" {
				inputItems = append(inputItems, map[string]any{
					"role":    "user",
					"content": fmt.Sprintf("[tool_output_missing_call_id] %v", output),
				})
				continue
			}

			pendingToolOutputs = append(pendingToolOutputs, pendingToolOutput{
				callID: callID,
				name:   name,
				output: output,
				index:  toolOutputIndex,
			})
			toolOutputIndex++
			continue
		}

		// Prefer mapping system/developer messages into `instructions`.
		if role == "system" || role == "developer" {
			if msg.Content == nil {
				continue
			}
			if msg.IsStringContent() {
				if s := strings.TrimSpace(msg.StringContent()); s != "" {
					instructionsParts = append(instructionsParts, s)
				}
				continue
			}
			parts := msg.ParseContent()
			var sb strings.Builder
			for _, part := range parts {
				if part.Type == dto.ContentTypeText && strings.TrimSpace(part.Text) != "" {
					if sb.Len() > 0 {
						sb.WriteString("\n")
					}
					sb.WriteString(part.Text)
				}
			}
			if s := strings.TrimSpace(sb.String()); s != "" {
				instructionsParts = append(instructionsParts, s)
			}
			continue
		}

		item := map[string]any{
			"role": role,
		}

		if msg.Content == nil {
			item["content"] = ""
			inputItems = append(inputItems, item)

			if role == "assistant" {
				for _, tc := range msg.ParseToolCalls() {
					if strings.TrimSpace(tc.ID) == "" {
						continue
					}
					if tc.Type != "" && tc.Type != "function" {
						continue
					}
					name := strings.TrimSpace(tc.Function.Name)
					if name == "" {
						continue
					}
					toolCallNamesByID[tc.ID] = name
					pendingFunctionCalls = append(pendingFunctionCalls, pendingFunctionCall{
						callID:    tc.ID,
						name:      name,
						arguments: tc.Function.Arguments,
						index:     functionCallIndex,
					})
					functionCallIndex++
				}
			}
			continue
		}

		if msg.IsStringContent() {
			item["content"] = msg.StringContent()
			inputItems = append(inputItems, item)

			if role == "assistant" {
				for _, tc := range msg.ParseToolCalls() {
					if strings.TrimSpace(tc.ID) == "" {
						continue
					}
					if tc.Type != "" && tc.Type != "function" {
						continue
					}
					name := strings.TrimSpace(tc.Function.Name)
					if name == "" {
						continue
					}
					toolCallNamesByID[tc.ID] = name
					pendingFunctionCalls = append(pendingFunctionCalls, pendingFunctionCall{
						callID:    tc.ID,
						name:      name,
						arguments: tc.Function.Arguments,
						index:     functionCallIndex,
					})
					functionCallIndex++
				}
			}
			continue
		}

		parts := msg.ParseContent()
		contentParts := make([]map[string]any, 0, len(parts))
		for _, part := range parts {
			switch part.Type {
			case dto.ContentTypeText:
				textType := "input_text"
				if role == "assistant" {
					textType = "output_text"
				}
				contentParts = append(contentParts, map[string]any{
					"type": textType,
					"text": part.Text,
				})
			case dto.ContentTypeImageURL:
				contentParts = append(contentParts, map[string]any{
					"type":      "input_image",
					"image_url": normalizeChatImageURLToString(part.ImageUrl),
				})
			case dto.ContentTypeInputAudio:
				contentParts = append(contentParts, map[string]any{
					"type":        "input_audio",
					"input_audio": part.InputAudio,
				})
			case dto.ContentTypeFile:
				contentParts = append(contentParts, map[string]any{
					"type": "input_file",
					"file": part.File,
				})
			case dto.ContentTypeVideoUrl:
				contentParts = append(contentParts, map[string]any{
					"type":      "input_video",
					"video_url": part.VideoUrl,
				})
			default:
				contentParts = append(contentParts, map[string]any{
					"type": part.Type,
				})
			}
		}
		item["content"] = contentParts
		inputItems = append(inputItems, item)

		if role == "assistant" {
			for _, tc := range msg.ParseToolCalls() {
				if strings.TrimSpace(tc.ID) == "" {
					continue
				}
				if tc.Type != "" && tc.Type != "function" {
					continue
				}
				name := strings.TrimSpace(tc.Function.Name)
				if name == "" {
					continue
				}
				toolCallNamesByID[tc.ID] = name
				pendingFunctionCalls = append(pendingFunctionCalls, pendingFunctionCall{
					callID:    tc.ID,
					name:      name,
					arguments: tc.Function.Arguments,
					index:     functionCallIndex,
				})
				functionCallIndex++
			}
		}
	}

	sort.SliceStable(pendingFunctionCalls, func(i, j int) bool {
		return pendingFunctionCalls[i].index < pendingFunctionCalls[j].index
	})
	sort.SliceStable(pendingToolOutputs, func(i, j int) bool {
		return pendingToolOutputs[i].index < pendingToolOutputs[j].index
	})

	remainingOutputsByID := make(map[string][]pendingToolOutput)
	for _, output := range pendingToolOutputs {
		mappedName := strings.TrimSpace(toolCallNamesByID[output.callID])
		if output.name == "" && mappedName != "" {
			output.name = mappedName
		}
		if output.name == "" {
			output.name = "unknown_function"
		}
		remainingOutputsByID[output.callID] = append(remainingOutputsByID[output.callID], output)
	}

	for _, call := range pendingFunctionCalls {
		inputItems = append(inputItems, map[string]any{
			"type":      "function_call",
			"call_id":   call.callID,
			"name":      call.name,
			"arguments": call.arguments,
		})
		outputs := remainingOutputsByID[call.callID]
		for _, output := range outputs {
			inputItems = append(inputItems, map[string]any{
				"type":    "function_call_output",
				"call_id": output.callID,
				"name":    output.name,
				"output":  output.output,
			})
		}
		delete(remainingOutputsByID, call.callID)
	}

	remainingCallIDs := make([]string, 0, len(remainingOutputsByID))
	for callID := range remainingOutputsByID {
		remainingCallIDs = append(remainingCallIDs, callID)
	}
	sort.Strings(remainingCallIDs)
	for _, callID := range remainingCallIDs {
		for _, output := range remainingOutputsByID[callID] {
			inputItems = append(inputItems, map[string]any{
				"type":    "function_call_output",
				"call_id": output.callID,
				"name":    output.name,
				"output":  output.output,
			})
		}
	}

	inputRaw, err := common.Marshal(inputItems)
	if err != nil {
		return nil, err
	}

	var instructionsRaw json.RawMessage
	if len(instructionsParts) > 0 {
		instructions := strings.Join(instructionsParts, "\n\n")
		instructionsRaw, _ = common.Marshal(instructions)
	}

	var toolsRaw json.RawMessage
	if req.Tools != nil {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			switch tool.Type {
			case "function":
				tools = append(tools, map[string]any{
					"type":        "function",
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  normalizeResponsesSchemaTypes(tool.Function.Parameters),
				})
			default:
				// Best-effort: keep original tool shape for unknown types.
				var m map[string]any
				if b, err := common.Marshal(tool); err == nil {
					_ = common.Unmarshal(b, &m)
				}
				if len(m) == 0 {
					m = map[string]any{"type": tool.Type}
				}
				tools = append(tools, m)
			}
		}
		toolsRaw, _ = common.Marshal(tools)
	}

	var toolChoiceRaw json.RawMessage
	if req.ToolChoice != nil {
		switch v := req.ToolChoice.(type) {
		case string:
			toolChoiceRaw, _ = common.Marshal(v)
		default:
			var m map[string]any
			if b, err := common.Marshal(v); err == nil {
				_ = common.Unmarshal(b, &m)
			}
			if m == nil {
				toolChoiceRaw, _ = common.Marshal(v)
			} else if t, _ := m["type"].(string); t == "function" {
				// Chat: {"type":"function","function":{"name":"..."}}
				// Responses: {"type":"function","name":"..."}
				if name, ok := m["name"].(string); ok && name != "" {
					toolChoiceRaw, _ = common.Marshal(map[string]any{
						"type": "function",
						"name": name,
					})
				} else if fn, ok := m["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok && name != "" {
						toolChoiceRaw, _ = common.Marshal(map[string]any{
							"type": "function",
							"name": name,
						})
					} else {
						toolChoiceRaw, _ = common.Marshal(v)
					}
				} else {
					toolChoiceRaw, _ = common.Marshal(v)
				}
			} else {
				toolChoiceRaw, _ = common.Marshal(v)
			}
		}
	}

	var parallelToolCallsRaw json.RawMessage
	if req.ParallelTooCalls != nil {
		parallelToolCallsRaw, _ = common.Marshal(*req.ParallelTooCalls)
	}

	textRaw := convertChatResponseFormatToResponsesText(req.ResponseFormat)

	maxOutputTokens := lo.FromPtrOr(req.MaxTokens, uint(0))
	maxCompletionTokens := lo.FromPtrOr(req.MaxCompletionTokens, uint(0))
	if maxCompletionTokens > maxOutputTokens {
		maxOutputTokens = maxCompletionTokens
	}
	// OpenAI Responses API rejects max_output_tokens < 16 when explicitly provided.
	//if maxOutputTokens > 0 && maxOutputTokens < 16 {
	//	maxOutputTokens = 16
	//}

	var topP *float64
	if req.TopP != nil {
		topP = common.GetPointer(lo.FromPtr(req.TopP))
	}

	out := &dto.OpenAIResponsesRequest{
		Model:             req.Model,
		Input:             inputRaw,
		Instructions:      instructionsRaw,
		Stream:            req.Stream,
		Temperature:       req.Temperature,
		Text:              textRaw,
		ToolChoice:        toolChoiceRaw,
		Tools:             toolsRaw,
		TopP:              topP,
		User:              req.User,
		ParallelToolCalls: parallelToolCallsRaw,
		Store:             req.Store,
		Metadata:          req.Metadata,
	}
	if req.MaxTokens != nil || req.MaxCompletionTokens != nil {
		out.MaxOutputTokens = lo.ToPtr(maxOutputTokens)
	}

	if req.ReasoningEffort != "" {
		out.Reasoning = &dto.Reasoning{
			Effort:  req.ReasoningEffort,
			Summary: "detailed",
		}
	}

	return out, nil
}
