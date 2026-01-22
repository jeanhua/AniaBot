package message

type NCrkey struct {
	Rkey string `json:"rkey"`
	TTL  string `json:"ttl"`
	Time uint   `json:"time"`
	Type uint   `json:"type"`
}
