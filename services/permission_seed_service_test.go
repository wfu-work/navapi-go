package services

import "testing"

func TestNavapiPermissionSeedsAreValid(t *testing.T) {
	guids := map[string]struct{}{}
	codes := map[string]struct{}{}
	for _, seed := range navapiAPIPermissionSeeds {
		if seed.Guid == "" {
			t.Fatal("permission seed guid is empty")
		}
		if len(seed.Guid) > 50 {
			t.Fatalf("permission seed guid %q exceeds 50 chars", seed.Guid)
		}
		if seed.Code == "" {
			t.Fatalf("permission seed %q code is empty", seed.Guid)
		}
		if seed.Path == "" || seed.Path[0] != '/' {
			t.Fatalf("permission seed %q path must start with /, got %q", seed.Guid, seed.Path)
		}
		if seed.Verb == "" {
			t.Fatalf("permission seed %q verb is empty", seed.Guid)
		}
		if _, ok := guids[seed.Guid]; ok {
			t.Fatalf("duplicate permission guid %q", seed.Guid)
		}
		if _, ok := codes[seed.Code]; ok {
			t.Fatalf("duplicate permission code %q", seed.Code)
		}
		guids[seed.Guid] = struct{}{}
		codes[seed.Code] = struct{}{}
	}
}

func TestNavapiUserPermissionSeedsCoverClientConsole(t *testing.T) {
	required := map[string]struct{}{
		"GET /models/list":            {},
		"GET /models/groups":          {},
		"GET /token/self/list":        {},
		"POST /token/self":            {},
		"PUT /token/self":             {},
		"DELETE /token/self/:id":      {},
		"GET /quota/self":             {},
		"GET /user-settings/self":     {},
		"PUT /user-settings/self":     {},
		"GET /usage/self/summary":     {},
		"GET /wallet/self":            {},
		"GET /wallet/self/records":    {},
		"GET /payment/self/list":      {},
		"POST /payment/create":        {},
		"GET /subscription/plans":     {},
		"GET /subscription/self/list": {},
		"GET /invitation/self/code":   {},
		"GET /checkin/self/status":    {},
		"POST /checkin/self":          {},
	}
	for _, seed := range navapiAPIPermissionSeeds {
		if !seed.User {
			continue
		}
		delete(required, seed.Verb+" "+seed.Path)
	}
	for key := range required {
		t.Fatalf("missing USER permission seed for %s", key)
	}
}
