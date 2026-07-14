package apis

import (
	"testing"

	"navapi-go/constants"
	"navapi-go/domains"
)

func TestBuildTokenBalanceResponse(t *testing.T) {
	t.Run("finite balance", func(t *testing.T) {
		result := buildTokenBalanceResponse(&domains.ApiToken{
			Name:                "primary",
			Status:              constants.StatusEnabled,
			BalanceAmountMicros: 12_500_000,
			UsedAmountMicros:    2_500_000,
		})

		if !result.IsActive || result.Unlimited {
			t.Fatalf("unexpected status: %+v", result)
		}
		if result.Balance == nil || *result.Balance != 12.5 {
			t.Fatalf("unexpected balance: %+v", result.Balance)
		}
		if result.Total == nil || *result.Total != 15 {
			t.Fatalf("unexpected total: %+v", result.Total)
		}
		if result.Used != 2.5 || result.Unit != "CNY" {
			t.Fatalf("unexpected usage: %+v", result)
		}
	})

	t.Run("unlimited balance", func(t *testing.T) {
		result := buildTokenBalanceResponse(&domains.ApiToken{
			Name:             "unlimited",
			Status:           constants.StatusEnabled,
			UnlimitedBalance: true,
			UsedAmountMicros: 3_000_000,
		})

		if !result.Unlimited || result.Balance != nil || result.Total != nil {
			t.Fatalf("unexpected unlimited balance: %+v", result)
		}
		if result.Used != 3 {
			t.Fatalf("unexpected used amount: %v", result.Used)
		}
	})
}
