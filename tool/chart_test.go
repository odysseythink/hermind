package tool

import (
	"encoding/json"
	"fmt"
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

func TestValidateDataset(t *testing.T) {
	tests := []struct {
		name        string
		dataset     string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid simple dataset",
			dataset:     `[{"name":"East","sales":1200},{"name":"West","sales":900}]`,
			shouldError: false,
		},
		{
			name:        "valid dataset with multiple metrics",
			dataset:     `[{"name":"Jan","revenue":4000,"cost":2400}]`,
			shouldError: false,
		},
		{
			name:        "invalid JSON",
			dataset:     `[{"name":"East","sales":1200}`,
			shouldError: true,
			errorMsg:    "JSON",
		},
		{
			name:        "missing name field",
			dataset:     `[{"value":1200}]`,
			shouldError: true,
			errorMsg:    "name",
		},
		{
			name:        "no numeric fields",
			dataset:     `[{"name":"East"}]`,
			shouldError: true,
			errorMsg:    "numeric",
		},
		{
			name:        "empty array",
			dataset:     `[]`,
			shouldError: true,
			errorMsg:    "empty",
		},
		{
			name:        "not an array",
			dataset:     `{"name":"East","value":1200}`,
			shouldError: true,
			errorMsg:    "array",
		},
	}

	for _, test := range tests {
		err := ValidateDataset(test.dataset)
		if (err != nil) != test.shouldError {
			t.Errorf("%s: error = %v, expected error = %v", test.name, err, test.shouldError)
		}
		if err != nil && test.shouldError && test.errorMsg != "" {
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.errorMsg)) {
				t.Errorf("%s: error should contain %q, got %q", test.name, test.errorMsg, err.Error())
			}
		}
	}
}

func TestDatasetSize(t *testing.T) {
	tests := []struct {
		dataset     string
		expectedLen int
		shouldError bool
	}{
		{`[{"name":"a","v":1}]`, 1, false},
		{`[{"name":"a","v":1},{"name":"b","v":2}]`, 2, false},
		{`[]`, 0, true},
		{`invalid`, 0, true},
	}

	for _, test := range tests {
		size, err := DatasetSize(test.dataset)
		if (err != nil) != test.shouldError {
			t.Errorf("DatasetSize(%q): error = %v, expected error = %v", test.dataset, err, test.shouldError)
		}
		if !test.shouldError && size != test.expectedLen {
			t.Errorf("DatasetSize(%q) = %d, expected %d", test.dataset, size, test.expectedLen)
		}
	}
}

func TestValidateChartInput(t *testing.T) {
	tests := []struct {
		name        string
		input       *ChartInput
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid input",
			input: &ChartInput{
				Type:    "bar",
				Title:   "Sales",
				Dataset: `[{"name":"Q1","val":1200}]`,
			},
			shouldError: false,
		},
		{
			name: "invalid chart type",
			input: &ChartInput{
				Type:    "invalid",
				Title:   "Sales",
				Dataset: `[{"name":"Q1","val":1200}]`,
			},
			shouldError: true,
			errorMsg:    "chart type",
		},
		{
			name: "empty title",
			input: &ChartInput{
				Type:    "bar",
				Title:   "",
				Dataset: `[{"name":"Q1","val":1200}]`,
			},
			shouldError: true,
			errorMsg:    "empty",
		},
		{
			name: "invalid dataset",
			input: &ChartInput{
				Type:    "bar",
				Title:   "Sales",
				Dataset: `{invalid}`,
			},
			shouldError: true,
			errorMsg:    "JSON",
		},
	}

	for _, test := range tests {
		err := ValidateChartInput(test.input)
		if (err != nil) != test.shouldError {
			t.Errorf("%s: error = %v, expected error = %v", test.name, err, test.shouldError)
		}
		if err != nil && test.shouldError && test.errorMsg != "" {
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.errorMsg)) {
				t.Errorf("%s: error should contain %q, got %q", test.name, test.errorMsg, err.Error())
			}
		}
	}
}

func TestDatasetSizeLimit(t *testing.T) {
	maxSize := 1000

	// Create dataset with exactly maxSize items
	var itemsMax []map[string]interface{}
	for i := 0; i < maxSize; i++ {
		itemsMax = append(itemsMax, map[string]interface{}{
			"name": fmt.Sprintf("item%d", i),
			"val":  float64(i),
		})
	}
	datasetMax, _ := json.Marshal(itemsMax)

	// Create dataset with maxSize+1 items
	var itemsOver []map[string]interface{}
	for i := 0; i < maxSize+1; i++ {
		itemsOver = append(itemsOver, map[string]interface{}{
			"name": fmt.Sprintf("item%d", i),
			"val":  float64(i),
		})
	}
	datasetOver, _ := json.Marshal(itemsOver)

	tests := []struct {
		name        string
		dataset     string
		shouldError bool
	}{
		{"exactly at limit", string(datasetMax), false},
		{"exceeds limit", string(datasetOver), true},
	}

	for _, test := range tests {
		size, _ := DatasetSize(test.dataset)
		if test.shouldError {
			if size <= maxSize {
				t.Errorf("%s: expected size > %d, got %d", test.name, maxSize, size)
			}
		}
	}
}
