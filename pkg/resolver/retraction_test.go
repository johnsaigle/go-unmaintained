package resolver

import (
	"strings"
	"testing"
)

func TestParseRetractions(t *testing.T) {
	tests := []struct {
		name     string
		goMod    string
		expected []RetractionRange
	}{
		{
			name: "single version retraction",
			goMod: `module example.com/mod

retract v1.0.0 // Published accidentally
`,
			expected: []RetractionRange{
				{Low: "v1.0.0", High: "v1.0.0", Reason: "Published accidentally"},
			},
		},
		{
			name: "range retraction",
			goMod: `module example.com/mod

retract [v1.1.0, v1.2.0] // Security vulnerability
`,
			expected: []RetractionRange{
				{Low: "v1.1.0", High: "v1.2.0", Reason: "Security vulnerability"},
			},
		},
		{
			name: "multiple retractions",
			goMod: `module example.com/mod

retract v1.0.0 // Published accidentally
retract [v1.5.0, v1.5.5] // Critical bug
`,
			expected: []RetractionRange{
				{Low: "v1.0.0", High: "v1.0.0", Reason: "Published accidentally"},
				{Low: "v1.5.0", High: "v1.5.5", Reason: "Critical bug"},
			},
		},
		{
			name: "retract block",
			goMod: `module example.com/mod

retract (
	v1.0.0 // Published accidentally
	[v1.5.0, v1.5.5] // Critical bug
)
`,
			expected: []RetractionRange{
				{Low: "v1.0.0", High: "v1.0.0", Reason: "Published accidentally"},
				{Low: "v1.5.0", High: "v1.5.5", Reason: "Critical bug"},
			},
		},
		{
			name: "comment above retract",
			goMod: `module example.com/mod

// This version has a critical security issue
// that allows remote code execution
retract v1.0.0
`,
			expected: []RetractionRange{
				{Low: "v1.0.0", High: "v1.0.0", Reason: "This version has a critical security issue that allows remote code execution"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.goMod)
			retractions, err := parseRetractions(reader)
			if err != nil {
				t.Fatalf("parseRetractions() error = %v", err)
			}

			if len(retractions) != len(tt.expected) {
				t.Errorf("got %d retractions, expected %d", len(retractions), len(tt.expected))
				return
			}

			for i, got := range retractions {
				expected := tt.expected[i]
				if got.Low != expected.Low {
					t.Errorf("retraction[%d].Low = %v, expected %v", i, got.Low, expected.Low)
				}
				if got.High != expected.High {
					t.Errorf("retraction[%d].High = %v, expected %v", i, got.High, expected.High)
				}
				if got.Reason != expected.Reason {
					t.Errorf("retraction[%d].Reason = %v, expected %v", i, got.Reason, expected.Reason)
				}
			}
		})
	}
}

func TestVersionInRange(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		low      string
		high     string
		expected bool
	}{
		{"exact match single", "v1.0.0", "v1.0.0", "v1.0.0", true},
		{"within range", "v1.1.5", "v1.1.0", "v1.2.0", true},
		{"below range", "v1.0.5", "v1.1.0", "v1.2.0", false},
		{"above range", "v1.3.0", "v1.1.0", "v1.2.0", false},
		{"at low boundary", "v1.1.0", "v1.1.0", "v1.2.0", true},
		{"at high boundary", "v1.2.0", "v1.1.0", "v1.2.0", true},
		{"invalid version", "invalid", "v1.0.0", "v1.1.0", false},
		{"invalid range", "v1.0.0", "invalid", "v1.1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := versionInRange(tt.version, tt.low, tt.high)
			if result != tt.expected {
				t.Errorf("versionInRange(%s, %s, %s) = %v, expected %v",
					tt.version, tt.low, tt.high, result, tt.expected)
			}
		})
	}
}
