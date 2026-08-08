package adminpanel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeanhua/AniaBot/common/plugininfo"
)

type fakeSkillDetailSource struct{}

func (fakeSkillDetailSource) SkillList() ([]plugininfo.SkillInfo, string, []string) {
	return nil, "", nil
}

func (fakeSkillDetailSource) SkillDelete(string) error {
	return nil
}

func (fakeSkillDetailSource) SkillUpload(string, []byte) error {
	return nil
}

func (fakeSkillDetailSource) SkillDetail(name string) (plugininfo.SkillDetail, error) {
	return plugininfo.SkillDetail{
		Name:        name,
		Description: "测试详情",
		Location:    "demo/SKILL.md",
		Content:     "# demo",
		Files: []plugininfo.SkillFileInfo{
			{Name: "reference.md", Kind: "reference", Size: 9, Content: "reference"},
			{Name: "run.sh", Kind: "extra", Size: 7},
		},
	}, nil
}

func TestHandleSkillDetail(t *testing.T) {
	s := &Server{opt: Options{Skills: fakeSkillDetailSource{}}}
	mux := http.NewServeMux()
	mux.Handle("GET /api/skills/{name}", http.HandlerFunc(s.handleSkillDetail))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/skills/demo", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got plugininfo.SkillDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.Name != "demo" || got.Content != "# demo" {
		t.Fatalf("detail = %+v", got)
	}
	if len(got.Files) != 2 || got.Files[0].Name != "reference.md" || got.Files[0].Content != "reference" {
		t.Fatalf("files = %+v", got.Files)
	}
}

func TestHandleSkillDetailUnsupported(t *testing.T) {
	s := &Server{opt: Options{Skills: skillSourceWithoutDetail{}}}
	mux := http.NewServeMux()
	mux.Handle("GET /api/skills/{name}", http.HandlerFunc(s.handleSkillDetail))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/skills/demo", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type skillSourceWithoutDetail struct{}

func (skillSourceWithoutDetail) SkillList() ([]plugininfo.SkillInfo, string, []string) {
	return nil, "", nil
}

func (skillSourceWithoutDetail) SkillDelete(string) error {
	return nil
}

func (skillSourceWithoutDetail) SkillUpload(string, []byte) error {
	return nil
}
