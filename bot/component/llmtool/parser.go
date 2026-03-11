package llmtool

import (
	"encoding/json"
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

	// 检查是否是 MCP 工具
	if mcpTool, ok := tool.(*MCPTool); ok {
		return mcpToolToOpenAITool(name, description, mcpTool.GetParameters())
	}

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

// mcpToolToOpenAITool 将 MCP 工具参数转换为 OpenAI 工具格式
func mcpToolToOpenAITool(name, description string, parameters json.RawMessage) llms.Tool {
	// MCP 的 parameters 已经是 JSON Schema 格式
	// 检查参数是否为空
	if len(parameters) == 0 {
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

	// 尝试解析并提取 properties 和 required
	var schema map[string]any
	if err := json.Unmarshal(parameters, &schema); err != nil {
		log.Printf("[MCP:%s] 解析参数定义失败: %v", name, err)
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

	// 确保有 type 字段
	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}

	// 确保有 properties 字段，避免LLM无法识别参数结构
	if _, ok := schema["properties"]; !ok {
		schema["properties"] = map[string]any{}
	}

	// 确保有 required 字段：
	// MCP服务器可能返回 null、缺失或非数组类型，统一兜底为空数组
	// 避免LLM因 required 缺失而不知道哪些参数是必填的，进而先试空参数
	switch v := schema["required"].(type) {
	case []any:
		// 正常情况，已有合法的 required 数组，保持不变
		_ = v
	case nil:
		// MCP服务器返回了 null 或字段不存在
		// 此时从 properties 中推断：将所有属性视为必填
		if props, ok := schema["properties"].(map[string]any); ok && len(props) > 0 {
			required := make([]string, 0, len(props))
			for k := range props {
				required = append(required, k)
			}
			schema["required"] = required
			log.Printf("[MCP:%s] required字段缺失，从properties推断必填参数: %v", name, required)
		} else {
			schema["required"] = []string{}
		}
	default:
		schema["required"] = []string{}
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
