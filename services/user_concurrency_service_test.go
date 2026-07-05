package services

import (
	"errors"
	"net/http"
	"testing"

	"navapi-go/domains"
)

func TestUserConcurrencyServiceAcquireReleaseUsesUserSettings(t *testing.T) {
	withUserSettingsTestDB(t)
	if _, err := UserSettingsServiceApp.Save("user-001", &domains.UserSettings{
		QuotaReminderEnabled:        true,
		PlatformAnnouncementEnabled: true,
		MaxConcurrency:              2,
		ExtraConfig:                 "{}",
	}); err != nil {
		t.Fatal(err)
	}

	releaseOne, err := UserConcurrencyServiceApp.Acquire("user-001")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOne()
	releaseTwo, err := UserConcurrencyServiceApp.Acquire("user-001")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseTwo()

	releaseThree, err := UserConcurrencyServiceApp.Acquire("user-001")
	if err == nil {
		releaseThree()
		t.Fatal("third acquire succeeded, want concurrency limit error")
	}
	var relayErr *RelayHTTPError
	if !errors.As(err, &relayErr) || relayErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("third acquire error = %v, want 429 RelayHTTPError", err)
	}

	releaseOne()
	releaseAfterFree, err := UserConcurrencyServiceApp.Acquire("user-001")
	if err != nil {
		t.Fatal(err)
	}
	releaseAfterFree()
}
