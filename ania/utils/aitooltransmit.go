package utils

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

func StructToOpenAITool(name, description string, input interface{}) llms.Tool {
	t := reflect.TypeOf(input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	propty := parseFields(t)

	return llms.Tool{
		Type: "object",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters:  propty,
		},
	}
}

func parseFields(t reflect.Type) map[string]Property {
	res := make(map[string]Property)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		name := strings.Split(jsonTag, ",")[0]

		prop := Property{
			Type:        goTypeToJSONType(field.Type),
			Description: field.Tag.Get("desc"),
		}

		if field.Type.Kind() == reflect.Struct {
			prop.Properties = parseFields(field.Type)
		}

		if field.Type.Kind() == reflect.Slice {
			elemType := field.Type.Elem()
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
