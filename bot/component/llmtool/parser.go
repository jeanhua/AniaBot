package llmtool

import (
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

func structToOpenAITool(name, description string, params any) llms.Tool {
	t := reflect.TypeOf(params)
	if t.Kind() == reflect.Ptr {
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

func parseFields(t reflect.Type) map[string]Property {
	res := map[string]Property{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]

		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
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
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			prop.Items = &Property{
				Type: goTypeToJSONType(elemType),
			}
			if elemType.Kind() == reflect.Struct {
				prop.Items.Properties = parseFields(elemType)
			}
		}
		res[name] = prop
	}
	return res
}

func goTypeToJSONType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
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
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" && !strings.Contains(jsonTag, "omitempty") {
			required = append(required, strings.Split(jsonTag, ",")[0])
		}
	}
	return required
}
