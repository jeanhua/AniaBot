package utils

import "time"

func GetFormattedTime() string {
	return time.Now().Format(time.RFC3339)
}
