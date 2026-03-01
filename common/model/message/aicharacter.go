package message

type AIChatacterResp struct {
	Type       string        `json:"type"`
	Characters []AIChatacter `json:"characters"`
}

type AIChatacter struct {
	CharacterID   string `json:"character_id"`
	CharacterName string `json:"character_name"`
	PreviewURL    string `json:"preview_url"`
}
