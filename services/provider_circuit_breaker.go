package services

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type providerCircuitSettings struct {
	Enabled          bool
	FailureThreshold int
	Cooldown         time.Duration
	MaxCooldown      time.Duration
}

type providerCircuitScope uint8

const (
	providerCircuitScopeGlobal providerCircuitScope = iota
	providerCircuitScopeEndpoint
)

const (
	defaultProviderNotFoundCooldown = 5 * time.Minute
	defaultProviderThrottleCooldown = time.Minute
)

type providerCircuitKey struct {
	ProviderGuid string
	ModelName    string
	Endpoint     string
	Scope        providerCircuitScope
}

type providerCircuitEntry struct {
	Failures      int
	OpenCount     int
	Generation    uint64
	OpenUntil     time.Time
	ProbeInFlight bool
	UpdatedAt     time.Time
}

type providerCircuitPermit struct {
	globalKey         providerCircuitKey
	endpointKey       providerCircuitKey
	halfOpenKeys      map[providerCircuitKey]struct{}
	generations       map[providerCircuitKey]uint64
	breakerGeneration uint64
	settings          providerCircuitSettings
	circuitDisabled   bool
}

type providerCircuitOutcomeKind uint8

const (
	providerCircuitHealthy providerCircuitOutcomeKind = iota
	providerCircuitIgnored
	providerCircuitGlobalFailure
	providerCircuitNotFound
	providerCircuitThrottled
)

type providerCircuitOutcome struct {
	Kind       providerCircuitOutcomeKind
	RetryAfter time.Duration
}

type ProviderCircuitBreaker struct {
	mu         sync.Mutex
	entries    map[providerCircuitKey]*providerCircuitEntry
	generation uint64
	now        func() time.Time
	settings   func() providerCircuitSettings
}

var ProviderCircuitBreakerApp = newProviderCircuitBreaker(providerCircuitSettingsFromOptions)

func newProviderCircuitBreaker(settings func() providerCircuitSettings) *ProviderCircuitBreaker {
	return &ProviderCircuitBreaker{
		entries:  make(map[providerCircuitKey]*providerCircuitEntry),
		now:      time.Now,
		settings: settings,
	}
}

func providerCircuitSettingsFromOptions() providerCircuitSettings {
	threshold := OptionServiceApp.Int64("relay.provider_failure_threshold", 2)
	cooldownSeconds := OptionServiceApp.Int64("relay.provider_cooldown_seconds", 30)
	maxCooldownSeconds := OptionServiceApp.Int64("relay.provider_max_cooldown_seconds", 600)
	if threshold <= 0 {
		threshold = 2
	}
	if cooldownSeconds <= 0 {
		cooldownSeconds = 30
	}
	if maxCooldownSeconds < cooldownSeconds {
		maxCooldownSeconds = cooldownSeconds
	}
	return providerCircuitSettings{
		Enabled:          OptionServiceApp.Bool("relay.provider_circuit_enabled", true),
		FailureThreshold: int(threshold),
		Cooldown:         time.Duration(cooldownSeconds) * time.Second,
		MaxCooldown:      time.Duration(maxCooldownSeconds) * time.Second,
	}
}

func (b *ProviderCircuitBreaker) TryAcquire(providerGuid string, modelName string, endpoint string) (*providerCircuitPermit, time.Duration, bool) {
	settings := b.settings()
	permit := &providerCircuitPermit{
		globalKey: providerCircuitKey{
			ProviderGuid: strings.TrimSpace(providerGuid),
			Scope:        providerCircuitScopeGlobal,
		},
		endpointKey: providerCircuitKey{
			ProviderGuid: strings.TrimSpace(providerGuid),
			ModelName:    strings.TrimSpace(modelName),
			Endpoint:     strings.TrimSpace(endpoint),
			Scope:        providerCircuitScopeEndpoint,
		},
		halfOpenKeys: make(map[providerCircuitKey]struct{}),
		generations:  make(map[providerCircuitKey]uint64),
		settings:     settings,
	}
	if !settings.Enabled || permit.globalKey.ProviderGuid == "" {
		permit.circuitDisabled = true
		return permit, 0, true
	}

	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	permit.breakerGeneration = b.generation
	b.pruneLocked(now)

	keys := []providerCircuitKey{permit.globalKey, permit.endpointKey}
	var retryAfter time.Duration
	for _, key := range keys {
		entry := b.entries[key]
		if entry == nil || entry.OpenUntil.IsZero() {
			continue
		}
		if entry.OpenUntil.After(now) {
			if wait := entry.OpenUntil.Sub(now); wait > retryAfter {
				retryAfter = wait
			}
			continue
		}
		if entry.ProbeInFlight && retryAfter < time.Second {
			retryAfter = time.Second
		}
	}
	if retryAfter > 0 {
		return nil, retryAfter, false
	}

	for _, key := range keys {
		entry := b.entries[key]
		if entry != nil {
			permit.generations[key] = entry.Generation
		}
		if entry == nil || entry.OpenUntil.IsZero() || entry.OpenUntil.After(now) {
			continue
		}
		entry.ProbeInFlight = true
		entry.UpdatedAt = now
		permit.halfOpenKeys[key] = struct{}{}
	}
	return permit, 0, true
}

func (b *ProviderCircuitBreaker) Record(permit *providerCircuitPermit, outcome providerCircuitOutcome) {
	if permit == nil || permit.circuitDisabled {
		return
	}
	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if permit.breakerGeneration != b.generation {
		return
	}

	switch outcome.Kind {
	case providerCircuitHealthy:
		b.clearLocked(permit, permit.globalKey)
		b.clearLocked(permit, permit.endpointKey)
	case providerCircuitIgnored:
		b.releaseHalfOpenLocked(permit, now)
	case providerCircuitGlobalFailure:
		b.releaseHalfOpenKeyLocked(permit, permit.endpointKey, now)
		b.failLocked(permit, permit.globalKey, permit.settings.FailureThreshold, permit.settings.Cooldown, now)
	case providerCircuitNotFound:
		b.releaseHalfOpenKeyLocked(permit, permit.globalKey, now)
		cooldown := minCircuitDuration(defaultProviderNotFoundCooldown, permit.settings.MaxCooldown)
		b.failLocked(permit, permit.endpointKey, 1, cooldown, now)
	case providerCircuitThrottled:
		b.releaseHalfOpenKeyLocked(permit, permit.endpointKey, now)
		cooldown := outcome.RetryAfter
		if cooldown <= 0 {
			cooldown = defaultProviderThrottleCooldown
		}
		cooldown = minCircuitDuration(cooldown, permit.settings.MaxCooldown)
		b.failLocked(permit, permit.globalKey, 1, cooldown, now)
	}
}

func (b *ProviderCircuitBreaker) Reset() {
	b.mu.Lock()
	b.entries = make(map[providerCircuitKey]*providerCircuitEntry)
	b.generation++
	b.mu.Unlock()
}

func (b *ProviderCircuitBreaker) failLocked(permit *providerCircuitPermit, key providerCircuitKey, threshold int, baseCooldown time.Duration, now time.Time) {
	entry := b.entries[key]
	observedGeneration := permit.generations[key]
	if entry != nil && entry.Generation != observedGeneration && !entry.OpenUntil.IsZero() {
		return
	}
	if entry == nil {
		entry = &providerCircuitEntry{}
		b.entries[key] = entry
	}
	_, wasHalfOpen := permit.halfOpenKeys[key]
	entry.ProbeInFlight = false
	entry.Failures++
	entry.UpdatedAt = now
	if !wasHalfOpen && entry.Failures < threshold {
		return
	}
	entry.Failures = threshold
	entry.OpenCount++
	entry.Generation++
	if entry.OpenCount < 1 {
		entry.OpenCount = 1
	}
	entry.OpenUntil = now.Add(exponentialCircuitCooldown(baseCooldown, permit.settings.MaxCooldown, entry.OpenCount))
}

func (b *ProviderCircuitBreaker) clearLocked(permit *providerCircuitPermit, key providerCircuitKey) {
	entry := b.entries[key]
	if entry == nil || entry.Generation != permit.generations[key] {
		return
	}
	delete(b.entries, key)
}

func (b *ProviderCircuitBreaker) releaseHalfOpenLocked(permit *providerCircuitPermit, now time.Time) {
	for key := range permit.halfOpenKeys {
		b.releaseHalfOpenKeyLocked(permit, key, now)
	}
}

func (b *ProviderCircuitBreaker) releaseHalfOpenKeyLocked(permit *providerCircuitPermit, key providerCircuitKey, now time.Time) {
	if _, ok := permit.halfOpenKeys[key]; !ok {
		return
	}
	if entry := b.entries[key]; entry != nil {
		if entry.Generation != permit.generations[key] {
			return
		}
		entry.ProbeInFlight = false
		entry.UpdatedAt = now
	}
}

func (b *ProviderCircuitBreaker) pruneLocked(now time.Time) {
	if len(b.entries) < 1024 {
		return
	}
	cutoff := now.Add(-24 * time.Hour)
	for key, entry := range b.entries {
		if !entry.ProbeInFlight && entry.UpdatedAt.Before(cutoff) {
			delete(b.entries, key)
		}
	}
}

func exponentialCircuitCooldown(base time.Duration, maximum time.Duration, openCount int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	if maximum < base {
		maximum = base
	}
	cooldown := base
	for i := 1; i < openCount && cooldown < maximum; i++ {
		if cooldown > maximum/2 {
			return maximum
		}
		cooldown *= 2
	}
	return minCircuitDuration(cooldown, maximum)
}

func minCircuitDuration(value time.Duration, maximum time.Duration) time.Duration {
	if maximum > 0 && value > maximum {
		return maximum
	}
	return value
}

func classifyProviderCircuitOutcome(requestContext context.Context, result *RelayResult, err error, now time.Time) providerCircuitOutcome {
	if isDownstreamStreamWriteError(err) || errors.Is(err, context.Canceled) || requestContext != nil && errors.Is(requestContext.Err(), context.Canceled) {
		return providerCircuitOutcome{Kind: providerCircuitIgnored}
	}
	if isUpstreamResponseLimitError(err) {
		if result == nil {
			return providerCircuitOutcome{Kind: providerCircuitHealthy}
		}
		err = nil
	}
	if err != nil {
		return providerCircuitOutcome{Kind: providerCircuitGlobalFailure}
	}
	if result == nil {
		return providerCircuitOutcome{Kind: providerCircuitGlobalFailure}
	}
	switch {
	case result.StatusCode == http.StatusNotFound:
		return providerCircuitOutcome{Kind: providerCircuitNotFound}
	case result.StatusCode == http.StatusTooManyRequests:
		return providerCircuitOutcome{
			Kind:       providerCircuitThrottled,
			RetryAfter: parseRetryAfter(result.Header.Get("Retry-After"), now),
		}
	case result.StatusCode == http.StatusRequestTimeout || result.StatusCode >= http.StatusInternalServerError:
		return providerCircuitOutcome{Kind: providerCircuitGlobalFailure}
	default:
		return providerCircuitOutcome{Kind: providerCircuitHealthy}
	}
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		if seconds > int64((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}

type downstreamStreamWriteError struct {
	err error
}

func (e *downstreamStreamWriteError) Error() string {
	return e.err.Error()
}

func (e *downstreamStreamWriteError) Unwrap() error {
	return e.err
}

func isDownstreamStreamWriteError(err error) bool {
	var writeErr *downstreamStreamWriteError
	return errors.As(err, &writeErr)
}
