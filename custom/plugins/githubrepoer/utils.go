package githubrepoer

import (
	"encoding/json"

	"github.com/go-resty/resty/v2"
)

func getRepoInfo(url string, compress bool) (string, error) {
	client := resty.New()
	optionstr, _ := json.Marshal(map[string]bool{
		"removeComments":     false,
		"removeEmptyLines":   false,
		"showLineNumbers":    false,
		"fileSummary":        true,
		"directoryStructure": true,
		"outputParsable":     false,
		"compress":           compress,
	})
	formData := map[string]string{
		"url":     url,
		"format":  "plain",
		"options": string(optionstr),
	}
	repoData := RepoData{}
	if _, err := client.R().SetFormData(formData).SetResult(&repoData).Post("https://api.repomix.com/api/pack"); err != nil {
		return "", err
	}
	return repoData.Content, nil
}

type RepoData struct {
	Content  string `json:"content"`
	Format   string `json:"format"`
	Metadata struct {
		Repository string `json:"repository"`
		Timestamp  string `json:"timestamp"`
		Summary    struct {
			TotalFiles      int `json:"totalFiles"`
			TotalCharacters int `json:"totalCharacters"`
			TotalTokens     int `json:"totalTokens"`
		} `json:"summary"`
	} `json:"metadata"`
}
