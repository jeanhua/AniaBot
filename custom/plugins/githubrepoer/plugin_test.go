package githubrepoer

import (
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestGetRepoInfo(t *testing.T) {
	result, err := getRepoInfo(resty.New(), "https://github.com/jeanhua/AniaBot", false, false, false, 100000, "", "")
	if err != nil {
		panic(err)
	}
	t.Log("成功获取", result)
}
