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

	// 检查是否是 MCP 工具
	if mcpTool, ok := tool.(*MCPTool); ok {
		schema := mcpTool.GetInputSchema()
		schemaBytes, _ := json.Marshal(schema)
		return mcpToolToOpenAITool(name, description, schemaBytes)
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
	switch v := schema["required"].(type) {
	case []any:
		// 正常情况，已有合法的 required 数组，保持不变
		_ = v
	case nil:
		// MCP服务器返回了 null 或字段不存在
		// 默认为空数组，让模型根据参数描述判断
		// 如果需要从 properties 中推断必填字段，可以检查每个属性的 schema
		schema["required"] = extractRequiredFromProperties(schema["properties"])
	default:
		schema["required"] = []string{}
	}

	// 增强参数描述：确保每个参数都有清晰的描述
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

// extractRequiredFromProperties 从 properties 中提取必填字段
// 检查每个属性的 schema，如果没有 default 值且不是 nullable，则视为必填
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

		// 如果属性没有 default 值，且不是 nullable，可能是必填的
		// 但为了安全起见，我们默认都设为可选，让模型根据描述判断
		// 只有明确标记的才算必填
		if _, hasDefault := propMap["default"]; !hasDefault {
			// 检查是否明确标记为必填（某些 MCP 实现可能在属性级别标记）
			if isRequired, ok := propMap["required"].(bool); ok && isRequired {
				required = append(required, name)
			}
		}
	}

	return required
}

// enhancePropertyDescriptions 增强参数描述，确保模型能理解参数用途
func enhancePropertyDescriptions(properties map[string]any) {
	for name, prop := range properties {
		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}

		// 如果缺少描述，添加基于参数名的提示
		if desc, ok := propMap["description"].(string); !ok || desc == "" {
			propMap["description"] = fmt.Sprintf("Parameter: %s", name)
		}

		// 如果有 enum，在描述中明确列出可选值
		if enum, ok := propMap["enum"].([]any); ok && len(enum) > 0 {
			desc := propMap["description"].(string)
			propMap["description"] = fmt.Sprintf("%s (allowed values: %v)", desc, enum)
		}

		// 递归处理嵌套对象
		if nestedProps, ok := propMap["properties"].(map[string]any); ok {
			enhancePropertyDescriptions(nestedProps)
		}

		// 处理数组项
		if items, ok := propMap["items"].(map[string]any); ok {
			if itemProps, ok := items["properties"].(map[string]any); ok {
				enhancePropertyDescriptions(itemProps)
			}
		}
	}
}
