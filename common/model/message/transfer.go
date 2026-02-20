package message

import (
	"encoding/json"
	"reflect"
	"strings"
)

func TransferTo[T any](s OB11Segment, target *T) bool {
	if !validateType[T](s.Type) {
		return false
	}
	dataBytes, err := json.Marshal(s.Data)
	if err != nil {
		return false
	}

	err = json.Unmarshal(dataBytes, target)
	return err == nil
}

var typeMapping = map[string]string{
	"text":   "TextMessage",
	"face":   "FaceMessage",
	"image":  "ImageMessage",
	"at":     "MentionMessage",
	"reply":  "ReplyMessage",
	"video":  "VideoMessage",
	"record": "RecordMessage",
	"json":   "JsonMessage",
	"music":  "MusicMessage",
}

func validateType[T any](segType string) bool {
	var t T
	tType := reflect.TypeOf(t)
	if tType.Kind() == reflect.Ptr {
		tType = tType.Elem()
	}
	structName := tType.Name()
	expectedStruct, ok := typeMapping[segType]
	if !ok {
		return true
	}
	return strings.HasSuffix(structName, expectedStruct)
}
