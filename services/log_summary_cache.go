package services

import (
	"fmt"
	"sync"
	"time"
)

const (
	usageSummaryCacheTTL        = 15 * time.Second
	usageSummaryCacheMaxEntries = 256
)

type usageSummaryCacheEntry struct {
	Value     UsageSummary
	ExpiresAt time.Time
}

var usageSummaryCache = struct {
	sync.RWMutex
	Entries map[string]usageSummaryCacheEntry
}{
	Entries: map[string]usageSummaryCacheEntry{},
}

func usageSummaryCacheKey(userGuid string, original UsageSummaryQuery, normalized UsageSummaryQuery) string {
	startKey := normalized.StartTime
	endKey := normalized.EndTime
	mode := "fixed"
	if original.EndTime <= 0 {
		mode = "rolling"
		bucketMS := int64(usageSummaryCacheTTL / time.Millisecond)
		if bucketMS <= 0 {
			bucketMS = 1
		}
		endKey = normalized.EndTime / bucketMS
	}
	return fmt.Sprintf("%s|%s|%d|%d|%d|%d", mode, userGuid, normalized.Days, normalized.TopN, startKey, endKey)
}

func usageSummaryCacheGet(key string, now time.Time) (UsageSummary, bool) {
	usageSummaryCache.RLock()
	entry, ok := usageSummaryCache.Entries[key]
	if ok && now.Before(entry.ExpiresAt) {
		value := cloneUsageSummary(entry.Value)
		usageSummaryCache.RUnlock()
		return value, true
	}
	usageSummaryCache.RUnlock()
	if ok {
		usageSummaryCache.Lock()
		if current, exists := usageSummaryCache.Entries[key]; exists && !now.Before(current.ExpiresAt) {
			delete(usageSummaryCache.Entries, key)
		}
		usageSummaryCache.Unlock()
	}
	return UsageSummary{}, false
}

func usageSummaryCacheSet(key string, summary UsageSummary, now time.Time) {
	usageSummaryCache.Lock()
	if usageSummaryCache.Entries == nil {
		usageSummaryCache.Entries = map[string]usageSummaryCacheEntry{}
	}
	if len(usageSummaryCache.Entries) >= usageSummaryCacheMaxEntries {
		pruneUsageSummaryCacheLocked(now)
	}
	usageSummaryCache.Entries[key] = usageSummaryCacheEntry{
		Value:     cloneUsageSummary(summary),
		ExpiresAt: now.Add(usageSummaryCacheTTL),
	}
	usageSummaryCache.Unlock()
}

func pruneUsageSummaryCacheLocked(now time.Time) {
	for key, entry := range usageSummaryCache.Entries {
		if !now.Before(entry.ExpiresAt) {
			delete(usageSummaryCache.Entries, key)
		}
	}
	for len(usageSummaryCache.Entries) >= usageSummaryCacheMaxEntries {
		for key := range usageSummaryCache.Entries {
			delete(usageSummaryCache.Entries, key)
			break
		}
	}
}

func resetUsageSummaryCache() {
	usageSummaryCache.Lock()
	usageSummaryCache.Entries = map[string]usageSummaryCacheEntry{}
	usageSummaryCache.Unlock()
}

func cloneUsageSummary(in UsageSummary) UsageSummary {
	out := in
	out.Series = cloneDailyUsageDataSlice(in.Series)
	out.SeriesByModel = cloneUsageNamedSeriesSlice(in.SeriesByModel)
	out.ByModel = cloneUsageDimensionStatSlice(in.ByModel)
	out.ByProvider = cloneUsageDimensionStatSlice(in.ByProvider)
	out.ByToken = cloneUsageDimensionStatSlice(in.ByToken)
	out.ByUser = cloneUsageDimensionStatSlice(in.ByUser)
	return out
}

func cloneDailyUsageDataSlice(in []DailyUsageData) []DailyUsageData {
	if in == nil {
		return nil
	}
	out := make([]DailyUsageData, len(in))
	copy(out, in)
	return out
}

func cloneUsageNamedSeriesSlice(in []UsageNamedSeries) []UsageNamedSeries {
	if in == nil {
		return nil
	}
	out := make([]UsageNamedSeries, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Data = cloneDailyUsageDataSlice(in[i].Data)
	}
	return out
}

func cloneUsageDimensionStatSlice(in []UsageDimensionStat) []UsageDimensionStat {
	if in == nil {
		return nil
	}
	out := make([]UsageDimensionStat, len(in))
	copy(out, in)
	return out
}
