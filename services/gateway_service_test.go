package services

import (
	"testing"
	"time"

	"navapi-go/domains"
)

func TestBuildPublicModelStatusesUsesFirstResponseSamples(t *testing.T) {
	end := time.Now()
	start := end.Add(-serviceStatusWindow)
	models := []domains.ModelMeta{{ModelName: "gpt-test", DisplayName: "GPT Test"}}
	rows := []serviceUsageBucketRow{
		{
			BucketIndex:     0,
			ModelName:       "gpt-test",
			Requests:        4,
			SuccessRequests: 3,
			ErrorRequests:   1,
			LatencyTotalMs:  600,
			LatencySamples:  2,
			LastRequestAt:   end.UnixMilli(),
		},
		{
			BucketIndex:     0,
			ModelName:       "disabled-model",
			Requests:        10,
			SuccessRequests: 10,
			LatencyTotalMs:  100,
			LatencySamples:  10,
		},
	}

	statuses := buildPublicModelStatuses(models, rows, start, end)
	if len(statuses) != 1 {
		t.Fatalf("expected one enabled model status, got %d", len(statuses))
	}
	status := statuses[0]
	if status.FirstResponseTimeMs != 300 {
		t.Fatalf("expected 300ms average first response, got %d", status.FirstResponseTimeMs)
	}
	if status.Requests != 4 {
		t.Fatalf("expected 4 real requests, got %d", status.Requests)
	}

	summary := summarizePublicServiceStatus(PublicServiceStatusSummary{EnabledModels: 1}, statuses)
	if summary.AvgFirstResponseTimeMs != 300 {
		t.Fatalf("expected 300ms summary first response, got %d", summary.AvgFirstResponseTimeMs)
	}
}
