package domains

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestModelMetaJSONDoesNotExposeID(t *testing.T) {
	model := ModelMeta{
		ModelName: "gpt-4o-mini",
	}
	model.Id = 12
	model.Guid = "model-guid"

	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, `"id"`) {
		t.Fatalf("model meta json exposes id: %s", body)
	}
	if !strings.Contains(body, `"guid":"model-guid"`) {
		t.Fatalf("model meta json does not expose guid: %s", body)
	}
}
