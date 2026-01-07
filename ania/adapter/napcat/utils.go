package napcat

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func generateMessageID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%s:%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s:%s", prefix, hex.EncodeToString(bytes))
}
