package alerts

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ---- Test helpers ----

// captureRouter records all routed groups.
type captureRouter struct {
	mu     sync.Mutex
	groups []*CorrelatedAlertGroup
}

func (r *captureRouter) Route(_ context.Context, group *CorrelatedAlertGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups = append(r.groups, group)
	return nil
}

func (r *captureRouter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.groups)
}

func (r *captureRouter) Last() *CorrelatedAlertGroup {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.groups) == 0 {
		return nil
	}
	return r.groups[len(r.groups)-1]
}

// errorRouter always returns an error from Route.
type errorRouter struct{}

func (e *errorRouter) Route(_ context.Context, _ *CorrelatedAlertGroup) error {
	return errors.New("routing failed")
}

// makeAlert creates a test alert with the given attributes.
func makeAlert(id, source, severity string, labels map[string]string) Alert {
	return Alert{
		ID:       id,
		Source:   source,
		Severity: severity,
		Title:    fmt.Sprintf("Alert %s", id),
		Status:   "firing",
		StartsAt: time.Now(),
		Labels:   labels,
	}
}

// ---- severityPriority tests ----

func TestSeverityPriority_KnownValues(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"critical", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
		{"", 0},
		{"CRITICAL", 0}, // case-sensitive
	}
	for _, tc := range tests {
		got := severityPriority(tc.input)
		if got != tc.want {
			t.Errorf("severityPriority(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestSeverityPriority_Ordering(t *testing.T) {
	if !(severityPriority("critical") > severityPriority("high")) {
		t.Error("critical should outrank high")
	}
	if !(severityPriority("high") > severityPriority("medium")) {
		t.Error("high should outrank medium")
	}
	if !(severityPriority("medium") > severityPriority("low")) {
		t.Error("medium should outrank low")
	}
	if !(severityPriority("low") > severityPriority("")) {
		t.Error("low should outrank unknown")
	}
}

// ---- Alert struct tests ----

func TestAlert_ZeroValue(t *testing.T) {
	var a Alert
	if a.Status != "" {
		t.Errorf("expected empty status, got %q", a.Status)
	}
	if a.EndsAt != nil {
		t.Error("expected nil EndsAt")
	}
}

func TestAlert_Resolved(t *testing.T) {
	now := time.Now()
	a := Alert{
		ID:       "a1",
		Status:   "resolved",
		StartsAt: now.Add(-5 * time.Minute),
		EndsAt:   &now,
	}
	if a.Status != "resolved" {
		t.Errorf("status = %q, want resolved", a.Status)
	}
	if a.EndsAt == nil {
		t.Error("EndsAt should not be nil for resolved alert")
	}
}

// ---- CorrelatedAlertGroup tests ----

func TestCorrelatedAlertGroup_ZeroValue(t *testing.T) {
	var g CorrelatedAlertGroup
	if g.Escalations != 0 {
		t.Errorf("expected 0 escalations, got %d", g.Escalations)
	}
}

// ---- Correlator construction ----

func TestNewCorrelator_NotNil(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	if c == nil {
		t.Fatal("NewCorrelator returned nil")
	}
}

func TestNewCorrelator_HasDefaultRules(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	if len(c.rules) == 0 {
		t.Error("expected default correlation rules, got none")
	}
}

// ---- generateCorrelationKey tests ----

func TestGenerateCorrelationKey_Deterministic(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	a := makeAlert("a1", "prometheus", "high", map[string]string{
		"service": "api",
	})
	k1 := c.generateCorrelationKey(a)
	k2 := c.generateCorrelationKey(a)
	if k1 != k2 {
		t.Errorf("correlation key is not deterministic: %q vs %q", k1, k2)
	}
}

func TestGenerateCorrelationKey_DifferentAlertsProduceDifferentKeys(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	a1 := makeAlert("a1", "prometheus", "high", map[string]string{"service": "api"})
	a2 := makeAlert("a2", "cloudwatch", "low", map[string]string{"service": "db"})
	if c.generateCorrelationKey(a1) == c.generateCorrelationKey(a2) {
		t.Error("different alerts should produce different correlation keys")
	}
}

func TestGenerateCorrelationKey_HasExpectedLength(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	a := makeAlert("a1", "prometheus", "high", nil)
	k := c.generateCorrelationKey(a)
	// sha256 truncated to 8 bytes = 16 hex chars
	if len(k) != 16 {
		t.Errorf("expected 16-char key, got %d: %q", len(k), k)
	}
}

// ---- matchesRule tests ----

func TestMatchesRule_NoLabels(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "test",
		MatchLabels: []string{"service", "environment"},
	}
	a := makeAlert("a1", "prometheus", "low", map[string]string{})
	if c.matchesRule(a, rule) {
		t.Error("alert with no matching labels should not match rule")
	}
}

func TestMatchesRule_OneMatchingLabel(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "test",
		MatchLabels: []string{"service", "environment"},
	}
	a := makeAlert("a1", "prometheus", "low", map[string]string{"service": "api"})
	if c.matchesRule(a, rule) {
		t.Error("alert with only one matching label should NOT match a rule requiring both labels")
	}
}

func TestMatchesRule_AllLabelsMatch(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "test",
		MatchLabels: []string{"service", "environment"},
	}
	a := makeAlert("a1", "prometheus", "low", map[string]string{
		"service":     "api",
		"environment": "prod",
	})
	if !c.matchesRule(a, rule) {
		t.Error("alert with all matching labels should match rule")
	}
}

func TestMatchesRule_NilLabels(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "test",
		MatchLabels: []string{"service"},
	}
	a := makeAlert("a1", "prometheus", "low", nil)
	if c.matchesRule(a, rule) {
		t.Error("nil labels should not match")
	}
}

// ---- ProcessAlert: single alert creates a group ----

func TestProcessAlert_SingleAlert_CreatesGroup(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	a := makeAlert("a1", "prometheus", "high", map[string]string{
		"service":     "payments",
		"environment": "prod",
	})

	if err := c.ProcessAlert(context.Background(), a); err != nil {
		t.Fatalf("ProcessAlert: %v", err)
	}

	if router.Len() != 1 {
		t.Errorf("expected 1 route call, got %d", router.Len())
	}

	group := router.Last()
	if group == nil {
		t.Fatal("group is nil")
	}
	if len(group.Alerts) != 1 {
		t.Errorf("expected 1 alert in group, got %d", len(group.Alerts))
	}
	if group.Status != "firing" {
		t.Errorf("group status = %q, want firing", group.Status)
	}
}

// ---- ProcessAlert: two alerts with same service group together ----

func TestProcessAlert_SameService_GroupsTogether(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	labels := map[string]string{
		"service":     "payments",
		"environment": "prod",
	}
	a1 := makeAlert("a1", "prometheus", "high", labels)
	a2 := makeAlert("a2", "prometheus", "medium", labels)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	// Both route calls should reference the same group ID.
	if router.Len() < 2 {
		t.Fatalf("expected at least 2 route calls, got %d", router.Len())
	}

	lastGroup := router.Last()
	if len(lastGroup.Alerts) < 2 {
		t.Errorf("expected 2 alerts in group, got %d", len(lastGroup.Alerts))
	}
}

// ---- ProcessAlert: severity escalation ----

func TestProcessAlert_SeverityEscalates_WhenHigherSeverityAdded(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	labels := map[string]string{
		"service":     "database",
		"environment": "prod",
	}
	a1 := makeAlert("a1", "prometheus", "medium", labels)
	a2 := makeAlert("a2", "prometheus", "critical", labels)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	group := router.Last()
	if group.Severity != "critical" {
		t.Errorf("group severity = %q, want critical (should escalate)", group.Severity)
	}
}

func TestProcessAlert_SeverityDoesNotDowngrade(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	labels := map[string]string{
		"service":     "api",
		"environment": "prod",
	}
	a1 := makeAlert("a1", "prometheus", "critical", labels)
	a2 := makeAlert("a2", "prometheus", "low", labels)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	group := router.Last()
	if group.Severity != "critical" {
		t.Errorf("group severity = %q, want critical (should not downgrade)", group.Severity)
	}
}

// ---- ProcessAlert: deduplication via correlation key ----

func TestProcessAlert_SameCorrelationKey_DoesNotCreateNewGroup(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	a1 := makeAlert("a1", "prometheus", "high", map[string]string{
		"service":     "api",
		"environment": "prod",
	})
	// Second alert with same correlation key.
	a2 := a1
	a2.ID = "a2"
	a2.CorrelationKey = c.generateCorrelationKey(a1)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}

	// Record groups before second call.
	groupsBefore := len(c.groups)

	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	if len(c.groups) != groupsBefore {
		t.Errorf("expected same group count after dedup, before=%d after=%d", groupsBefore, len(c.groups))
	}
}

// ---- ProcessAlert: alerts with no matching rule create single-alert group ----

func TestProcessAlert_NoMatchingRule_CreatesSingleAlertGroup(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	// Alert with no labels — no default rule will match.
	a := makeAlert("a1", "custom", "low", map[string]string{})

	if err := c.ProcessAlert(context.Background(), a); err != nil {
		t.Fatalf("ProcessAlert: %v", err)
	}

	if router.Len() != 1 {
		t.Fatalf("expected 1 route call, got %d", router.Len())
	}

	group := router.Last()
	if len(group.Alerts) != 1 {
		t.Errorf("expected 1 alert in single-alert group, got %d", len(group.Alerts))
	}
	if group.Title != a.Title {
		t.Errorf("group title = %q, want %q", group.Title, a.Title)
	}
}

// ---- ProcessAlert: router error propagates ----

func TestProcessAlert_RouterError_Propagates(t *testing.T) {
	c := NewCorrelator(&errorRouter{})
	a := makeAlert("a1", "prometheus", "high", map[string]string{"service": "api"})
	err := c.ProcessAlert(context.Background(), a)
	if err == nil {
		t.Error("expected error from router, got nil")
	}
}

// ---- ProcessAlert: cloud resource grouping ----

func TestProcessAlert_SameCloudResource_GroupsTogether(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	labels := map[string]string{
		"cloud":       "aws",
		"resource_id": "i-0abc123",
	}
	a1 := makeAlert("a1", "cloudwatch", "high", labels)
	a2 := makeAlert("a2", "cloudwatch", "medium", labels)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	last := router.Last()
	if len(last.Alerts) < 2 {
		t.Errorf("expected 2 alerts for same cloud resource, got %d", len(last.Alerts))
	}
}

// ---- ProcessAlert: cross-provider same resource ----

func TestProcessAlert_CrossProvider_SameResource_GroupsTogether(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	// Two alerts about the same logical resource from different sources.
	a1 := makeAlert("a1", "cloudwatch", "high", map[string]string{
		"cloud":       "aws",
		"resource_id": "db-prod-001",
	})
	a2 := makeAlert("a2", "prometheus", "medium", map[string]string{
		"cloud":       "aws",
		"resource_id": "db-prod-001",
	})

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	last := router.Last()
	if len(last.Alerts) < 2 {
		t.Errorf("expected 2 alerts grouped from cross-provider, got %d", len(last.Alerts))
	}
}

// ---- ProcessAlert: same host grouping ----

func TestProcessAlert_SameHost_GroupsTogether(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	labels := map[string]string{"host": "prod-web-01"}
	a1 := makeAlert("a1", "prometheus", "high", labels)
	a2 := makeAlert("a2", "prometheus", "critical", labels)

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	last := router.Last()
	if len(last.Alerts) < 2 {
		t.Errorf("expected 2 alerts for same host, got %d", len(last.Alerts))
	}
}

// ---- ProcessAlert: different hosts do NOT group ----

func TestProcessAlert_DifferentHosts_DoNotGroup(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	a1 := makeAlert("a1", "prometheus", "high", map[string]string{"host": "web-01"})
	a2 := makeAlert("a2", "prometheus", "high", map[string]string{"host": "web-02"})

	if err := c.ProcessAlert(context.Background(), a1); err != nil {
		t.Fatalf("ProcessAlert a1: %v", err)
	}
	if err := c.ProcessAlert(context.Background(), a2); err != nil {
		t.Fatalf("ProcessAlert a2: %v", err)
	}

	if len(c.groups) < 2 {
		t.Errorf("expected 2 separate groups for different hosts, got %d", len(c.groups))
	}
}

// ---- generateGroupKey tests ----

func TestGenerateGroupKey_Deterministic(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "same-service",
		MatchLabels: []string{"service"},
	}
	a := makeAlert("a1", "prometheus", "high", map[string]string{"service": "api"})
	k1 := c.generateGroupKey(a, rule)
	k2 := c.generateGroupKey(a, rule)
	if k1 != k2 {
		t.Errorf("group key is not deterministic: %q vs %q", k1, k2)
	}
}

func TestGenerateGroupKey_DifferentLabels_DifferentKeys(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	rule := CorrelationRule{
		Name:        "same-service",
		MatchLabels: []string{"service"},
	}
	a1 := makeAlert("a1", "prometheus", "high", map[string]string{"service": "api"})
	a2 := makeAlert("a2", "prometheus", "high", map[string]string{"service": "db"})
	if c.generateGroupKey(a1, rule) == c.generateGroupKey(a2, rule) {
		t.Error("different service labels should produce different group keys")
	}
}

// ---- cleanup tests ----

func TestCleanup_RemovesResolvedOldGroups(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	// Manually inject a resolved group that is old enough to be cleaned.
	old := time.Now().Add(-2 * time.Hour)
	group := &CorrelatedAlertGroup{
		ID:        "old-group",
		Status:    "resolved",
		UpdatedAt: old,
		Alerts: []Alert{
			{CorrelationKey: "key-old"},
		},
	}
	c.groups["old-group"] = group
	c.alertToGroup["key-old"] = "old-group"

	c.cleanup()

	if _, exists := c.groups["old-group"]; exists {
		t.Error("old resolved group should have been removed")
	}
	if _, exists := c.alertToGroup["key-old"]; exists {
		t.Error("alert mapping for old group should have been removed")
	}
}

func TestCleanup_PreservesRecentResolvedGroups(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	// Recently resolved group should not be cleaned.
	group := &CorrelatedAlertGroup{
		ID:        "recent-group",
		Status:    "resolved",
		UpdatedAt: time.Now().Add(-10 * time.Minute), // within 1-hour cutoff
		Alerts:    []Alert{{CorrelationKey: "key-recent"}},
	}
	c.groups["recent-group"] = group
	c.alertToGroup["key-recent"] = "recent-group"

	c.cleanup()

	if _, exists := c.groups["recent-group"]; !exists {
		t.Error("recently resolved group should be preserved")
	}
}

func TestCleanup_PreservesFiringGroups(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	group := &CorrelatedAlertGroup{
		ID:        "firing-group",
		Status:    "firing",
		UpdatedAt: time.Now().Add(-2 * time.Hour), // old but firing
		Alerts:    []Alert{{CorrelationKey: "key-firing"}},
	}
	c.groups["firing-group"] = group
	c.alertToGroup["key-firing"] = "firing-group"

	c.cleanup()

	if _, exists := c.groups["firing-group"]; !exists {
		t.Error("firing group should never be cleaned")
	}
}

// ---- addToGroup: missing group returns error ----

func TestAddToGroup_MissingGroup_ReturnsError(t *testing.T) {
	c := NewCorrelator(&captureRouter{})
	a := makeAlert("a1", "prometheus", "high", nil)
	err := c.addToGroup(context.Background(), "nonexistent-group", a)
	if err == nil {
		t.Error("expected error for missing group ID, got nil")
	}
}

// ---- Concurrent processing ----

func TestProcessAlert_Concurrent_NoPanic(t *testing.T) {
	router := &captureRouter{}
	c := NewCorrelator(router)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := makeAlert(
				fmt.Sprintf("a%d", i),
				"prometheus",
				"high",
				map[string]string{
					"service":     fmt.Sprintf("svc-%d", i%5), // 5 distinct services
					"environment": "prod",
				},
			)
			_ = c.ProcessAlert(context.Background(), a)
		}(i)
	}
	wg.Wait()

	// All 50 alerts should be in groups.
	c.mu.RLock()
	total := 0
	for _, g := range c.groups {
		total += len(g.Alerts)
	}
	c.mu.RUnlock()

	if total != 50 {
		t.Errorf("expected 50 total alerts across groups, got %d", total)
	}
}

// ---- CorrelationRule GroupTitle function ----

func TestCorrelationRule_DefaultGroupTitle_SameService(t *testing.T) {
	rules := defaultCorrelationRules()
	var sameService CorrelationRule
	for _, r := range rules {
		if r.Name == "same-service" {
			sameService = r
			break
		}
	}
	if sameService.Name == "" {
		t.Fatal("same-service rule not found in defaults")
	}

	a := makeAlert("a1", "prometheus", "high", map[string]string{
		"service":     "api",
		"environment": "prod",
	})
	title := sameService.GroupTitle(a)
	if title == "" {
		t.Error("GroupTitle returned empty string")
	}
}

func TestCorrelationRule_DefaultGroupTitle_SameHost_UsesHost(t *testing.T) {
	rules := defaultCorrelationRules()
	var sameHost CorrelationRule
	for _, r := range rules {
		if r.Name == "same-host" {
			sameHost = r
			break
		}
	}
	if sameHost.Name == "" {
		t.Fatal("same-host rule not found in defaults")
	}

	a := makeAlert("a1", "prometheus", "high", map[string]string{"host": "prod-web-01"})
	title := sameHost.GroupTitle(a)
	expected := "Host: prod-web-01"
	if title != expected {
		t.Errorf("GroupTitle = %q, want %q", title, expected)
	}
}

func TestCorrelationRule_DefaultGroupTitle_SameInstance(t *testing.T) {
	rules := defaultCorrelationRules()
	var sameInstance CorrelationRule
	for _, r := range rules {
		if r.Name == "same-instance" {
			sameInstance = r
			break
		}
	}

	a := makeAlert("a1", "prometheus", "high", map[string]string{"instance": "10.0.0.1:9090"})
	title := sameInstance.GroupTitle(a)
	expected := "Instance: 10.0.0.1:9090"
	if title != expected {
		t.Errorf("GroupTitle = %q, want %q", title, expected)
	}
}

func TestDefaultCorrelationRules_Count(t *testing.T) {
	rules := defaultCorrelationRules()
	if len(rules) != 4 {
		t.Errorf("expected 4 default rules, got %d", len(rules))
	}
}

func TestDefaultCorrelationRules_AllHaveNames(t *testing.T) {
	for _, r := range defaultCorrelationRules() {
		if r.Name == "" {
			t.Error("rule with empty name found")
		}
	}
}

func TestDefaultCorrelationRules_AllHaveTimeWindow(t *testing.T) {
	for _, r := range defaultCorrelationRules() {
		if r.TimeWindow <= 0 {
			t.Errorf("rule %q has non-positive time window: %v", r.Name, r.TimeWindow)
		}
	}
}

func TestDefaultCorrelationRules_AllHavePriority(t *testing.T) {
	for _, r := range defaultCorrelationRules() {
		if r.Priority <= 0 {
			t.Errorf("rule %q has non-positive priority: %d", r.Name, r.Priority)
		}
	}
}
