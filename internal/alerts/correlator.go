// Package alerts provides intelligent alert correlation and routing.
package alerts

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// Alert represents an incoming alert from any source
type Alert struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`      // prometheus, cloudwatch, azure-monitor
	Severity    string            `json:"severity"`    // critical, high, medium, low
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Status      string            `json:"status"`    // firing, resolved
	StartsAt    time.Time         `json:"starts_at"`
	EndsAt      *time.Time        `json:"ends_at,omitempty"`

	// Correlation
	CorrelationKey string `json:"correlation_key"`
	GroupID        string `json:"group_id,omitempty"`
}

// CorrelatedAlertGroup represents a group of related alerts
type CorrelatedAlertGroup struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Severity    string    `json:"severity"` // Highest severity in group
	Alerts      []Alert   `json:"alerts"`
	RootCause   *Alert    `json:"root_cause,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Status      string    `json:"status"`
	Runbook     string    `json:"runbook,omitempty"`
	Escalations int       `json:"escalations"`
}

// Correlator correlates related alerts to reduce noise
type Correlator struct {
	mu           sync.RWMutex
	groups       map[string]*CorrelatedAlertGroup
	alertToGroup map[string]string
	rules        []CorrelationRule
	router       AlertRouter
}

// CorrelationRule defines how alerts are correlated
type CorrelationRule struct {
	Name        string
	Description string
	MatchLabels []string          // Labels that must match
	TimeWindow  time.Duration     // Alerts within this window are grouped
	Priority    int               // Higher priority rules are checked first
	GroupTitle  func(Alert) string // Function to generate group title
}

// AlertRouter routes correlated alerts to the appropriate destination
type AlertRouter interface {
	Route(ctx context.Context, group *CorrelatedAlertGroup) error
}

// NewCorrelator creates a new alert correlator
func NewCorrelator(router AlertRouter) *Correlator {
	c := &Correlator{
		groups:       make(map[string]*CorrelatedAlertGroup),
		alertToGroup: make(map[string]string),
		router:       router,
		rules:        defaultCorrelationRules(),
	}

	// Start cleanup goroutine
	go c.cleanupLoop()

	return c
}

// defaultCorrelationRules returns standard correlation rules
func defaultCorrelationRules() []CorrelationRule {
	return []CorrelationRule{
		{
			Name:        "same-service",
			Description: "Group alerts for the same service",
			MatchLabels: []string{"service", "environment"},
			TimeWindow:  5 * time.Minute,
			Priority:    10,
			GroupTitle: func(a Alert) string {
				return fmt.Sprintf("%s - %s", a.Labels["service"], a.Labels["environment"])
			},
		},
		{
			Name:        "same-host",
			Description: "Group alerts for the same host/instance",
			MatchLabels: []string{"instance", "host"},
			TimeWindow:  5 * time.Minute,
			Priority:    5,
			GroupTitle: func(a Alert) string {
				if host, ok := a.Labels["host"]; ok {
					return fmt.Sprintf("Host: %s", host)
				}
				return fmt.Sprintf("Instance: %s", a.Labels["instance"])
			},
		},
		{
			Name:        "same-cloud-resource",
			Description: "Group alerts for the same cloud resource",
			MatchLabels: []string{"cloud", "resource_id"},
			TimeWindow:  10 * time.Minute,
			Priority:    8,
			GroupTitle: func(a Alert) string {
				return fmt.Sprintf("%s - %s", a.Labels["cloud"], a.Labels["resource_id"])
			},
		},
	}
}

// ProcessAlert processes an incoming alert
func (c *Correlator) ProcessAlert(ctx context.Context, alert Alert) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Generate correlation key
	if alert.CorrelationKey == "" {
		alert.CorrelationKey = c.generateCorrelationKey(alert)
	}

	// Check if alert belongs to existing group
	if groupID, exists := c.alertToGroup[alert.CorrelationKey]; exists {
		return c.addToGroup(ctx, groupID, alert)
	}

	// Find matching rule and create/join group
	for _, rule := range c.rules {
		if c.matchesRule(alert, rule) {
			groupKey := c.generateGroupKey(alert, rule)

			if group, exists := c.groups[groupKey]; exists {
				return c.addToGroup(ctx, group.ID, alert)
			}

			// Create new group
			return c.createGroup(ctx, alert, rule, groupKey)
		}
	}

	// No matching rule - create single-alert group
	return c.createSingleAlertGroup(ctx, alert)
}

// generateCorrelationKey creates a unique key for alert deduplication
func (c *Correlator) generateCorrelationKey(alert Alert) string {
	data := fmt.Sprintf("%s:%s:%s:%s",
		alert.Source,
		alert.Title,
		alert.Labels["service"],
		alert.Labels["instance"],
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// generateGroupKey creates a key for grouping alerts
func (c *Correlator) generateGroupKey(alert Alert, rule CorrelationRule) string {
	data := rule.Name
	for _, label := range rule.MatchLabels {
		if v, ok := alert.Labels[label]; ok {
			data += ":" + v
		}
	}
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// matchesRule checks if an alert matches a correlation rule
func (c *Correlator) matchesRule(alert Alert, rule CorrelationRule) bool {
	matchCount := 0
	for _, label := range rule.MatchLabels {
		if _, ok := alert.Labels[label]; ok {
			matchCount++
		}
	}
	// Require at least one matching label
	return matchCount > 0
}

// createGroup creates a new correlation group
func (c *Correlator) createGroup(ctx context.Context, alert Alert, rule CorrelationRule, groupKey string) error {
	group := &CorrelatedAlertGroup{
		ID:        groupKey,
		Title:     rule.GroupTitle(alert),
		Severity:  alert.Severity,
		Alerts:    []Alert{alert},
		StartsAt:  alert.StartsAt,
		UpdatedAt: time.Now(),
		Status:    "firing",
	}

	c.groups[groupKey] = group
	c.alertToGroup[alert.CorrelationKey] = groupKey

	// Route the new group
	return c.router.Route(ctx, group)
}

// addToGroup adds an alert to an existing group
func (c *Correlator) addToGroup(ctx context.Context, groupID string, alert Alert) error {
	group, exists := c.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	// Add alert
	group.Alerts = append(group.Alerts, alert)
	group.UpdatedAt = time.Now()

	// Update severity if new alert is more severe
	if severityPriority(alert.Severity) > severityPriority(group.Severity) {
		group.Severity = alert.Severity
	}

	// Update correlation mapping
	c.alertToGroup[alert.CorrelationKey] = groupID

	// Route updated group
	return c.router.Route(ctx, group)
}

// createSingleAlertGroup creates a group for an uncorrelated alert
func (c *Correlator) createSingleAlertGroup(ctx context.Context, alert Alert) error {
	group := &CorrelatedAlertGroup{
		ID:        alert.CorrelationKey,
		Title:     alert.Title,
		Severity:  alert.Severity,
		Alerts:    []Alert{alert},
		StartsAt:  alert.StartsAt,
		UpdatedAt: time.Now(),
		Status:    "firing",
	}

	c.groups[group.ID] = group
	c.alertToGroup[alert.CorrelationKey] = group.ID

	return c.router.Route(ctx, group)
}

// cleanupLoop periodically removes resolved groups
func (c *Correlator) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes old resolved groups
func (c *Correlator) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-1 * time.Hour)

	for id, group := range c.groups {
		if group.Status == "resolved" && group.UpdatedAt.Before(cutoff) {
			// Remove group
			delete(c.groups, id)

			// Remove alert mappings
			for _, alert := range group.Alerts {
				delete(c.alertToGroup, alert.CorrelationKey)
			}
		}
	}
}

// severityPriority returns a numeric priority for severity comparison
func severityPriority(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

