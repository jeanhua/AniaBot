package githubrepoer

import (
	"testing"
)

func TestGetRepoInfo(t *testing.T) {
	result, err := getRepoInfo("https://github.com/jeanhua/AniaBot", false, 100000)
	if err != nil {
		panic(err)
	}
	t.Log("成功获取", result)
}
