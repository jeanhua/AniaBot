package llmtool

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
}

func structToOpenAITool(tool Tool) llms.Tool {
	name := tool.Name()
	description := tool.Description()
	params := tool.Params()

	if mcpTool, ok := tool.(*MCPTool); ok {
		schema := mcpTool.GetInputSchema()
		schemaBytes, _ := json.Marshal(schema)
		return mcpToolToOpenAITool(name, description, schemaBytes)
	}

	t := reflect.TypeOf(params)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": parseFields(t),
				"required":   getRequiredFields(t),
			},
		},
	}
}

func mcpToolToOpenAITool(name, description string, parameters json.RawMessage) llms.Tool {
	emptyTool := func() llms.Tool {
		return llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        name,
				Description: description,
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
					"required":   []string{},
				},
			},
		}
	}

	if len(parameters) == 0 {
		return emptyTool()
	}

	var schema map[string]any
	if err := json.Unmarshal(parameters, &schema); err != nil {
		log.Printf("[MCP:%s] 解析参数定义失败: %v", name, err)
		return emptyTool()
	}

	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]any{}
	}

	switch v := schema["required"].(type) {
	case []any:
		_ = v
	case nil:
		schema["required"] = extractRequiredFromProperties(schema["properties"])
	default:
		schema["required"] = []string{}
	}

	if props, ok := schema["properties"].(map[string]any); ok {
		enhancePropertyDescriptions(props)
	}

	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  schema,
		},
	}
}

func parseFields(t reflect.Type) map[string]Property {
	res := map[string]Property{}
	for _, field := range reflect.VisibleFields(t) {
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]
		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		prop := Property{
			Type:        goTypeToJSONType(fieldType),
			Description: field.Tag.Get("desc"),
		}

		if fieldType.Kind() == reflect.Struct {
			prop.Properties = parseFields(fieldType)
		}

		if fieldType.Kind() == reflect.Slice {
			elemType := fieldType.Elem()
			if elemType.Kind() == reflect.Pointer {
				elemType = elemType.Elem()
			}
			prop.Items = &Property{Type: goTypeToJSONType(elemType)}
			if elemType.Kind() == reflect.Struct {
				prop.Items.Properties = parseFields(elemType)
			}
		}
		res[name] = prop
	}
	return res
}

func goTypeToJSONType(t reflect.Type) string {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int64, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice:
		return "array"
	case reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

func getRequiredFields(t reflect.Type) []string {
	required := []string{}
	for _, field := range reflect.VisibleFields(t) {
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" && !strings.Contains(jsonTag, "omitempty") {
			required = append(required, strings.Split(jsonTag, ",")[0])
		}
	}
	return required
}

func extractRequiredFromProperties(properties any) []string {
	props, ok := properties.(map[string]any)
	if !ok || len(props) == 0 {
		return []string{}
	}
	required := make([]string, 0)
	for name, prop := range props {
		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}
		if _, hasDefault := propMap["default"]; !hasDefault {
			if isRequired, ok := propMap["required"].(bool); ok && isRequired {
				required = append(required, name)
			}
		}
	}
	return required
}

func enhancePropertyDescriptions(properties map[string]any) {
	for name, prop := range properties {
		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}
		if desc, ok := propMap["description"].(string); !ok || desc == "" {
			propMap["description"] = fmt.Sprintf("Parameter: %s", name)
		}
		if enum, ok := propMap["enum"].([]any); ok && len(enum) > 0 {
			desc := propMap["description"].(string)
			propMap["description"] = fmt.Sprintf("%s (allowed values: %v)", desc, enum)
		}
		if nestedProps, ok := propMap["properties"].(map[string]any); ok {
			enhancePropertyDescriptions(nestedProps)
		}
		if items, ok := propMap["items"].(map[string]any); ok {
			if itemProps, ok := items["properties"].(map[string]any); ok {
				enhancePropertyDescriptions(itemProps)
			}
		}
	}
}
