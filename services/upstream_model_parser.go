package services

import (
	"encoding/json"
	"sort"
	"strings"

	"navapi-go/constants"
)

func parseModelIDs(providerType string, body []byte) []string {
	switch providerType {
	case constants.ProviderTypeGemini:
		return parseGeminiModelIDs(body)
	default:
		return parseDataModelIDs(body)
	}
}

func parseDataModelIDs(body []byte) []string {
	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			models = append(models, item.ID)
			continue
		}
		if item.Name != "" {
			models = append(models, item.Name)
			continue
		}
		if item.DisplayName != "" {
			models = append(models, item.DisplayName)
		}
	}
	return models
}

func parseGeminiModelIDs(body []byte) []string {
	var payload struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	models := make([]string, 0, len(payload.Models))
	for _, item := range payload.Models {
		name := strings.TrimPrefix(item.Name, "models/")
		if name != "" {
			models = append(models, name)
			continue
		}
		if item.DisplayName != "" {
			models = append(models, item.DisplayName)
		}
	}
	return models
}

func uniqueSorted(models []string) []string {
	set := map[string]struct{}{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			set[model] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for model := range set {
		out = append(out, model)
	}
	sort.Strings(out)
	return out
}
