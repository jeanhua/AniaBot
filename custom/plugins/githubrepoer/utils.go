package githubrepoer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

func getRepoInfo(url string, compress, delComment, delEmptyLine bool, maxToken int, include, exclude string) (string, error) {
	client := resty.New()
	opt := map[string]any{
		"removeComments":     delComment,
		"removeEmptyLines":   delEmptyLine,
		"showLineNumbers":    false,
		"fileSummary":        true,
		"directoryStructure": true,
		"outputParsable":     false,
		"compress":           compress,
	}
	if include != "" {
		opt["includePatterns"] = include
	}
	if exclude != "" {
		opt["ignorePatterns"] = exclude
	}
	optionstr, _ := json.Marshal(opt)
	formData := map[string]string{
		"url":     url,
		"format":  "plain",
		"options": string(optionstr),
	}
	repoData := RepoData{}
	if _, err := client.R().SetFormData(formData).SetResult(&repoData).Post("https://api.repomix.com/api/pack"); err != nil {
		return "", err
	}
	if repoData.Metadata.Summary.TotalTokens > maxToken {
		return "", OutOfContextError
	}
	if repoData.Content == "" {
		return "", fmt.Errorf("empty result")
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

func parseCmd(args []string) work {
	w := work{}
	for _, arg := range args {
		switch arg {
		case "--compress":
			w.compress = true
		case "--del-comment":
			w.delComment = true
		case "--del-emptyline":
			w.delEmptyLine = true
		}

		if strings.HasPrefix(arg, "--include=") {
			w.include = strings.TrimPrefix(arg, "--include=")
		} else if strings.HasPrefix(arg, "--exclude=") {
			w.include = strings.TrimPrefix(arg, "--exclude=")
		}
	}

	return w
}
