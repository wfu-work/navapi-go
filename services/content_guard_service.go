package services

import (
	"net/http"
	"strings"
)

func ValidateSensitiveWords(body []byte) error {
	rawWords := OptionServiceApp.Get("relay.sensitive_words", "")
	if strings.TrimSpace(rawWords) == "" {
		return nil
	}
	content := strings.ToLower(string(body))
	for _, word := range splitCSV(rawWords) {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" {
			continue
		}
		if strings.Contains(content, word) {
			return &RelayHTTPError{StatusCode: http.StatusBadRequest, Message: "request contains sensitive content"}
		}
	}
	return nil
}
