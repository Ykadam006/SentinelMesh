package main

import "testing"

func TestWorstStatus(t *testing.T) {
	cases := []struct {
		breaches map[string]string
		want     string
	}{
		{map[string]string{}, "HEALTHY"},
		{map[string]string{"latency": "WARNING"}, "DEGRADED"},
		{map[string]string{"latency": "WARNING", "error_rate": "CRITICAL"}, "INCIDENT"},
	}
	for _, c := range cases {
		if got := worstStatus(c.breaches); got != c.want {
			t.Errorf("worstStatus(%v) = %s, want %s", c.breaches, got, c.want)
		}
	}
}

func TestErrorBudgetRemaining(t *testing.T) {
	// 99.9% target allows 0.1% downtime; 99.95% availability burns half of it
	if got := calculateErrorBudgetRemaining("payment-service", 99.95); got < 49.9 || got > 50.1 {
		t.Errorf("budget = %.2f, want 50", got)
	}
	if got := calculateErrorBudgetRemaining("payment-service", 98.0); got != 0 {
		t.Errorf("budget = %.2f, want clamped to 0", got)
	}
}
