package llmtool

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// TestExtractRequiredFromPropertiesDeterministic 验证 MCP schema 缺顶层 required 时，
// 从 properties map 推导的 required 数组顺序确定（按名称排序）：
// required 是 []string 切片，JSON 序列化保持原序，map 随机遍历会直接打失
// 上游 prompt 前缀缓存（与 v3.5.0 修复的 tools 数组顺序问题同类）。
func TestExtractRequiredFromPropertiesDeterministic(t *testing.T) {
	props := map[string]any{
		"query": map[string]any{"type": "string", "required": true},
		"limit": map[string]any{"type": "number", "required": true, "default": 10},
		"page":  map[string]any{"type": "number", "required": true},
		"note":  map[string]any{"type": "string"},
	}
	first := extractRequiredFromProperties(props)
	// limit 有 default 不算必填；query/page 必填
	want := []string{"page", "query"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("extractRequiredFromProperties = %v, want %v", first, want)
	}
	for round := range 10 {
		if got := extractRequiredFromProperties(props); !reflect.DeepEqual(got, first) {
			t.Fatalf("round %d: not deterministic: %v != %v", round, got, first)
		}
	}
}

// TestMCPToolToOpenAIToolRequiredSorted 验证完整链路：MCP inputSchema 经
// mcpToolToOpenAITool 转出的工具定义，其 JSON 中的 required 数组有序。
func TestMCPToolToOpenAIToolRequiredSorted(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"query": {"type": "string", "required": true, "description": "查询词"},
			"page":  {"type": "number", "required": true, "description": "页码"},
			"limit": {"type": "number", "description": "条数"}
		}
	}`
	var got []string
	for round := range 10 {
		td := mcpToolToOpenAITool("search", "搜索", json.RawMessage(schema))
		data, err := json.Marshal(td)
		if err != nil {
			t.Fatalf("round %d: marshal: %v", round, err)
		}
		var parsed struct {
			Function struct {
				Parameters struct {
					Required []string `json:"required"`
				} `json:"parameters"`
			} `json:"function"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("round %d: unmarshal: %v", round, err)
		}
		req := parsed.Function.Parameters.Required
		if round == 0 {
			got = req
			if !sort.StringsAreSorted(got) {
				t.Fatalf("required 未排序: %v", got)
			}
			continue
		}
		if !reflect.DeepEqual(req, got) {
			t.Fatalf("round %d: required 数组顺序不稳定: %v != %v", round, req, got)
		}
	}
}
