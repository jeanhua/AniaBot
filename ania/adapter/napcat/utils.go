package napcat

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func generateMessageID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
