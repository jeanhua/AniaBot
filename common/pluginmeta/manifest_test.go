package pluginmeta

import (
	"strings"
	"testing"
)

func validManifest() *Manifest {
	return &Manifest{
		ID:          "example",
		Name:        "示例插件",
		Description: "测试",
		Author:      "jeanhua",
		Version:     "1.0.0",
		APIVersion:  1,
	}
}

func TestValidateOK(t *testing.T) {
	m := validManifest()
	if err := m.Validate(); err != nil {
		t.Fatalf("应通过校验: %v", err)
	}
	if m.Constructor() != DefaultConstructor || m.ReadmeName() != DefaultReadme {
		t.Fatalf("默认值未补齐: constructor=%q readme=%q", m.Constructor(), m.ReadmeName())
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{"空 ID", func(m *Manifest) { m.ID = "" }, "ID 非法"},
		{"非法 ID", func(m *Manifest) { m.ID = "Abc/.." }, "ID 非法"},
		{"缺 name", func(m *Manifest) { m.Name = " " }, "name 必填"},
		{"缺 description", func(m *Manifest) { m.Description = "" }, "description 必填"},
		{"缺 author", func(m *Manifest) { m.Author = "" }, "author 必填"},
		{"缺 version", func(m *Manifest) { m.Version = "" }, "version 必填"},
		{"api 不兼容", func(m *Manifest) { m.APIVersion = APIVersion + 1 }, "不兼容"},
		{"icon 路径穿越", func(m *Manifest) { m.Icon = "../x.png" }, "icon 必须是文件名"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validManifest()
			c.edit(m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("期望错误包含 %q, got %v", c.want, err)
			}
		})
	}
}

func TestImportPath(t *testing.T) {
	got := ImportPath("hello")
	want := "github.com/jeanhua/AniaBot/custom/plugins/hello"
	if got != want {
		t.Fatalf("ImportPath = %q, want %q", got, want)
	}
}
