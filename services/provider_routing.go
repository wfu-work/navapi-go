package services

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"navapi-go/constants"
	"navapi-go/domains"

	"gorm.io/gorm"
)

type ProviderRouteCandidate struct {
	Provider       domains.VendorMeta
	GroupGuid      string
	GroupName      string
	Strategy       string
	Sort           int
	Priority       int
	Weight         int
	MaxConcurrency int
}

type ProviderRoutePlan struct {
	GroupGuid  string
	GroupName  string
	Strategy   string
	ModelName  string
	Endpoint   string
	Candidates []ProviderRouteCandidate
}

type ProviderRouteLease struct {
	router         *ProviderRouter
	key            string
	once           sync.Once
	InflightBefore int
}

func (l *ProviderRouteLease) Release() {
	if l == nil || l.router == nil || l.key == "" {
		return
	}
	l.once.Do(func() {
		l.router.mu.Lock()
		if current := l.router.inflight[l.key]; current <= 1 {
			delete(l.router.inflight, l.key)
		} else {
			l.router.inflight[l.key] = current - 1
		}
		l.router.mu.Unlock()
	})
}

type ProviderRouter struct {
	mu            sync.Mutex
	roundRobin    map[string]uint64
	currentWeight map[string]map[string]int64
	inflight      map[string]int
}

var ProviderRouterApp = newProviderRouter()

func newProviderRouter() *ProviderRouter {
	return &ProviderRouter{
		roundRobin:    make(map[string]uint64),
		currentWeight: make(map[string]map[string]int64),
		inflight:      make(map[string]int),
	}
}

func (s *ProviderService) FindRoutePlanForEndpointAndType(modelName, group string, providerType string, endpointPath string) (*ProviderRoutePlan, error) {
	providers, err := s.enabledProviders(providerType)
	if err != nil {
		return nil, err
	}
	eligible := make([]domains.VendorMeta, 0, len(providers))
	for _, provider := range providers {
		if len(splitCSV(provider.Models)) > 0 && !containsString(splitCSV(provider.Models), modelName) {
			continue
		}
		if !providerSupportsEndpoint(&provider, endpointPath) {
			continue
		}
		eligible = append(eligible, provider)
	}
	profile, err := ModelServiceApp.WithDB(s.DB()).RoutingProfileForGroup(group)
	if err != nil {
		return nil, err
	}
	plan := &ProviderRoutePlan{
		GroupGuid: profile.GroupGuid,
		GroupName: profile.GroupName,
		Strategy:  normalizeProviderRoutingStrategy(profile.RoutingStrategy),
		ModelName: strings.TrimSpace(modelName),
		Endpoint:  strings.TrimSpace(endpointPath),
	}
	if profile.ProviderScope == constants.ModelGroupProviderScopeSelected {
		byGuid := make(map[string]domains.VendorMeta, len(eligible))
		for _, provider := range eligible {
			byGuid[provider.Guid] = provider
		}
		for _, route := range profile.ProviderRoutes {
			if !route.RoutingEnabled {
				continue
			}
			provider, ok := byGuid[route.ProviderGuid]
			if !ok {
				continue
			}
			plan.Candidates = append(plan.Candidates, routeCandidate(plan, provider, route))
		}
	} else {
		for index, provider := range eligible {
			plan.Candidates = append(plan.Candidates, routeCandidate(plan, provider, domains.ModelGroupProviderRoute{
				ProviderGuid:   provider.Guid,
				Sort:           index,
				Weight:         100,
				RoutingEnabled: true,
			}))
		}
	}
	if len(plan.Candidates) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return plan, nil
}

func routeCandidate(plan *ProviderRoutePlan, provider domains.VendorMeta, route domains.ModelGroupProviderRoute) ProviderRouteCandidate {
	weight := route.Weight
	if weight <= 0 {
		weight = 100
	}
	return ProviderRouteCandidate{
		Provider:       provider,
		GroupGuid:      plan.GroupGuid,
		GroupName:      plan.GroupName,
		Strategy:       plan.Strategy,
		Sort:           route.Sort,
		Priority:       max(0, route.Priority),
		Weight:         weight,
		MaxConcurrency: max(0, route.MaxConcurrency),
	}
}

func (r *ProviderRouter) Order(plan *ProviderRoutePlan) []ProviderRouteCandidate {
	if plan == nil || len(plan.Candidates) == 0 {
		return nil
	}
	candidates := append([]ProviderRouteCandidate(nil), plan.Candidates...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority < candidates[j].Priority
		}
		return candidates[i].Sort < candidates[j].Sort
	})
	ordered := make([]ProviderRouteCandidate, 0, len(candidates))
	for start := 0; start < len(candidates); {
		end := start + 1
		for end < len(candidates) && candidates[end].Priority == candidates[start].Priority {
			end++
		}
		tier := candidates[start:end]
		key := routeStateKey(plan, candidates[start].Priority)
		switch plan.Strategy {
		case constants.ProviderRoutingRoundRobin:
			tier = r.roundRobinOrder(key, tier)
		case constants.ProviderRoutingWeightedRoundRobin:
			tier = r.weightedRoundRobinOrder(key, tier)
		case constants.ProviderRoutingLeastInflight:
			tier = r.leastInflightOrder(key, tier)
		}
		ordered = append(ordered, tier...)
		start = end
	}
	return ordered
}

func (r *ProviderRouter) TryAcquire(candidate ProviderRouteCandidate) (*ProviderRouteLease, bool) {
	key := providerInflightKey(candidate)
	r.mu.Lock()
	current := r.inflight[key]
	if candidate.MaxConcurrency > 0 && current >= candidate.MaxConcurrency {
		r.mu.Unlock()
		return &ProviderRouteLease{InflightBefore: current}, false
	}
	r.inflight[key] = current + 1
	r.mu.Unlock()
	return &ProviderRouteLease{router: r, key: key, InflightBefore: current}, true
}

func (r *ProviderRouter) Inflight(candidate ProviderRouteCandidate) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inflight[providerInflightKey(candidate)]
}

func (r *ProviderRouter) Reset() {
	r.mu.Lock()
	r.roundRobin = make(map[string]uint64)
	r.currentWeight = make(map[string]map[string]int64)
	r.inflight = make(map[string]int)
	r.mu.Unlock()
}

func (r *ProviderRouter) ResetScheduling(groupGuid string, groupName string) {
	prefixes := make([]string, 0, 2)
	if groupGuid = strings.TrimSpace(groupGuid); groupGuid != "" {
		prefixes = append(prefixes, groupGuid+"|")
	}
	if groupName = strings.TrimSpace(groupName); groupName != "" {
		prefixes = append(prefixes, groupName+"|")
	}
	if len(prefixes) == 0 {
		return
	}
	r.mu.Lock()
	for key := range r.roundRobin {
		if hasAnyPrefix(key, prefixes) {
			delete(r.roundRobin, key)
		}
	}
	for key := range r.currentWeight {
		if hasAnyPrefix(key, prefixes) {
			delete(r.currentWeight, key)
		}
	}
	r.mu.Unlock()
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func (r *ProviderRouter) roundRobinOrder(key string, candidates []ProviderRouteCandidate) []ProviderRouteCandidate {
	if len(candidates) <= 1 {
		return append([]ProviderRouteCandidate(nil), candidates...)
	}
	r.mu.Lock()
	start := int(r.roundRobin[key] % uint64(len(candidates)))
	r.roundRobin[key]++
	r.mu.Unlock()
	return rotateRouteCandidates(candidates, start)
}

func (r *ProviderRouter) weightedRoundRobinOrder(key string, candidates []ProviderRouteCandidate) []ProviderRouteCandidate {
	if len(candidates) <= 1 {
		return append([]ProviderRouteCandidate(nil), candidates...)
	}
	r.mu.Lock()
	state := r.currentWeight[key]
	if state == nil {
		state = make(map[string]int64, len(candidates))
		r.currentWeight[key] = state
	}
	present := make(map[string]struct{}, len(candidates))
	total := int64(0)
	selected := 0
	best := int64(0)
	for index, candidate := range candidates {
		guid := candidate.Provider.Guid
		present[guid] = struct{}{}
		weight := int64(max(1, candidate.Weight))
		total += weight
		state[guid] += weight
		if index == 0 || state[guid] > best {
			selected = index
			best = state[guid]
		}
	}
	state[candidates[selected].Provider.Guid] -= total
	for guid := range state {
		if _, ok := present[guid]; !ok {
			delete(state, guid)
		}
	}
	r.mu.Unlock()
	return rotateRouteCandidates(candidates, selected)
}

func (r *ProviderRouter) leastInflightOrder(key string, candidates []ProviderRouteCandidate) []ProviderRouteCandidate {
	if len(candidates) <= 1 {
		return append([]ProviderRouteCandidate(nil), candidates...)
	}
	r.mu.Lock()
	start := int(r.roundRobin[key] % uint64(len(candidates)))
	r.roundRobin[key]++
	rotated := rotateRouteCandidates(candidates, start)
	inflight := make(map[string]int, len(rotated))
	for _, candidate := range rotated {
		inflight[candidate.Provider.Guid] = r.inflight[providerInflightKey(candidate)]
	}
	r.mu.Unlock()
	sort.SliceStable(rotated, func(i, j int) bool {
		left := int64(inflight[rotated[i].Provider.Guid]) * int64(max(1, rotated[j].Weight))
		right := int64(inflight[rotated[j].Provider.Guid]) * int64(max(1, rotated[i].Weight))
		return left < right
	})
	return rotated
}

func rotateRouteCandidates(candidates []ProviderRouteCandidate, start int) []ProviderRouteCandidate {
	if len(candidates) == 0 {
		return nil
	}
	start %= len(candidates)
	ordered := make([]ProviderRouteCandidate, 0, len(candidates))
	ordered = append(ordered, candidates[start:]...)
	ordered = append(ordered, candidates[:start]...)
	return ordered
}

func routeStateKey(plan *ProviderRoutePlan, priority int) string {
	group := strings.TrimSpace(plan.GroupGuid)
	if group == "" {
		group = strings.TrimSpace(plan.GroupName)
	}
	return strings.Join([]string{group, plan.ModelName, plan.Endpoint, plan.Strategy, strconv.Itoa(priority)}, "|")
}

func providerInflightKey(candidate ProviderRouteCandidate) string {
	group := strings.TrimSpace(candidate.GroupGuid)
	if group == "" {
		group = strings.TrimSpace(candidate.GroupName)
	}
	return group + "|" + candidate.Provider.Guid
}
