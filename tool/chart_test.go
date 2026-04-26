package tool

import (
	"strings"
	"testing"
)

func TestValidChartType(t *testing.T) {
	tests := []struct {
		chartType string
		expected  bool
	}{
		{"bar", true},
		{"line", true},
		{"pie", true},
		{"area", true},
		{"composed", true},
		{"scatter", true},
		{"radar", true},
		{"radialBar", true},
		{"treemap", true},
		{"funnel", true},
		{"invalid", false},
		{"", false},
		{"BAR", false}, // case-sensitive
	}

	for _, test := range tests {
		result := ValidChartType(test.chartType)
		if result != test.expected {
			t.Errorf("ValidChartType(%q) = %v, expected %v", test.chartType, result, test.expected)
		}
	}
}

func TestValidateChartTypeInput(t *testing.T) {
	tests := []struct {
		chartType   string
		shouldError bool
	}{
		{"bar", false},
		{"line", false},
		{"invalid", true},
		{"", true},
	}

	for _, test := range tests {
		err := ValidateChartTypeInput(test.chartType)
		if (err != nil) != test.shouldError {
			t.Errorf("ValidateChartTypeInput(%q): error = %v, expected error = %v", test.chartType, err, test.shouldError)
		}
		if err != nil && test.shouldError {
			expected := "chart type must be one of:"
			if !strings.Contains(err.Error(), expected) {
				t.Errorf("Error message should contain %q, got %q", expected, err.Error())
			}
		}
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		title       string
		shouldError bool
	}{
		{"Sales Chart", false},
		{"Q1 Revenue", false},
		{"", true},
		{"   ", true},
		{"\t\n", true},
		{"Valid Title With Spaces", false},
	}

	for _, test := range tests {
		err := ValidateTitle(test.title)
		if (err != nil) != test.shouldError {
			t.Errorf("ValidateTitle(%q): error = %v, expected error = %v", test.title, err, test.shouldError)
		}
		if err != nil && test.shouldError && !strings.Contains(err.Error(), "empty") {
			t.Errorf("Error should mention empty title, got: %v", err)
		}
	}
}
