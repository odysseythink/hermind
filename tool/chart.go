package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ChartType represents the supported chart types.
type ChartType string

const (
	ChartTypeArea       ChartType = "area"
	ChartTypeBar        ChartType = "bar"
	ChartTypeLine       ChartType = "line"
	ChartTypeComposed   ChartType = "composed"
	ChartTypeScatter    ChartType = "scatter"
	ChartTypePie        ChartType = "pie"
	ChartTypeRadar      ChartType = "radar"
	ChartTypeRadialBar  ChartType = "radialBar"
	ChartTypeTreemap    ChartType = "treemap"
	ChartTypeFunnel     ChartType = "funnel"
)

var chartTypeValues = []ChartType{
	ChartTypeArea,
	ChartTypeBar,
	ChartTypeLine,
	ChartTypeComposed,
	ChartTypeScatter,
	ChartTypePie,
	ChartTypeRadar,
	ChartTypeRadialBar,
	ChartTypeTreemap,
	ChartTypeFunnel,
}

var validChartTypes = func() map[ChartType]bool {
	m := make(map[ChartType]bool)
	for _, t := range chartTypeValues {
		m[t] = true
	}
	return m
}()

const MaxDatasetSize = 1000

// ChartInput is the request payload for the chart tool.
type ChartInput struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Dataset string `json:"dataset"`
	Caption string `json:"caption,omitempty"`
}

// ChartOutput is the response payload from the chart tool.
type ChartOutput struct {
	Success bool   `json:"success"`
	Type    string `json:"type,omitempty"`
	Title   string `json:"title,omitempty"`
	Dataset string `json:"dataset,omitempty"`
	Message string `json:"message"`
}

// ValidChartType checks if a chart type string is valid.
func ValidChartType(t string) bool {
	return validChartTypes[ChartType(t)]
}

// AllChartTypes returns a comma-separated list of valid chart types.
func AllChartTypes() string {
	strs := make([]string, len(chartTypeValues))
	for i, t := range chartTypeValues {
		strs[i] = string(t)
	}
	return strings.Join(strs, ", ")
}

// ValidateTitle checks that title is non-empty and non-whitespace.
func ValidateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return errors.New("chart title cannot be empty")
	}
	return nil
}

// ValidateChartTypeInput checks that the chart type is valid.
func ValidateChartTypeInput(t string) error {
	if !ValidChartType(t) {
		return errors.New("chart type must be one of: " + AllChartTypes())
	}
	return nil
}

// ValidateDataset checks that dataset is valid JSON array with required structure.
func ValidateDataset(datasetStr string) error {
	var data []interface{}
	if err := json.Unmarshal([]byte(datasetStr), &data); err != nil {
		// Check if the error is due to non-array input
		if err.Error() == "json: cannot unmarshal object into Go value of type []interface {}" {
			return errors.New("dataset must be a JSON array, not an object")
		}
		return fmt.Errorf("invalid dataset JSON: %w", err)
	}

	if len(data) == 0 {
		return errors.New("dataset cannot be empty")
	}

	// Check that all items are objects with "name" field and at least one numeric field
	for i, item := range data {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("dataset item %d: must be an object", i)
		}

		// Check for "name" field
		if _, hasName := obj["name"]; !hasName {
			return fmt.Errorf("dataset item %d: missing required 'name' field", i)
		}

		// Check for at least one numeric field (other than "name")
		hasNumericField := false
		for k, v := range obj {
			if k == "name" {
				continue
			}
			switch v.(type) {
			case float64:
				hasNumericField = true
				break
			}
		}

		if !hasNumericField {
			return fmt.Errorf("dataset item %d: must have at least one numeric field", i)
		}
	}

	return nil
}

// DatasetSize returns the number of records in the dataset.
func DatasetSize(datasetStr string) (int, error) {
	var data []interface{}
	if err := json.Unmarshal([]byte(datasetStr), &data); err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, errors.New("dataset is empty")
	}
	return len(data), nil
}

// ValidateChartInput checks all chart parameters and returns an error if invalid.
func ValidateChartInput(input *ChartInput) error {
	// Validate type
	if err := ValidateChartTypeInput(input.Type); err != nil {
		return err
	}

	// Validate title
	if err := ValidateTitle(input.Title); err != nil {
		return err
	}

	// Validate dataset structure
	if err := ValidateDataset(input.Dataset); err != nil {
		return err
	}

	// Validate dataset size
	size, err := DatasetSize(input.Dataset)
	if err != nil {
		return err
	}
	if size > MaxDatasetSize {
		return fmt.Errorf("dataset exceeds maximum size. Limit to %d records (found %d)", MaxDatasetSize, size)
	}

	return nil
}

// HandleChart is the handler function for the chart tool.
func HandleChart(ctx context.Context, raw json.RawMessage) (string, error) {
	var input ChartInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return ToolError("invalid arguments: " + err.Error()), nil
	}

	// Validate all inputs
	if err := ValidateChartInput(&input); err != nil {
		return ToolError(err.Error()), nil
	}

	// Build successful response
	output := ChartOutput{
		Success: true,
		Type:    input.Type,
		Title:   input.Title,
		Dataset: input.Dataset,
		Message: "Chart generated successfully",
	}

	result, _ := json.Marshal(output)
	return string(result), nil
}

// RegisterChart registers the chart tool in the registry.
func RegisterChart(reg *Registry) {
	reg.Register(&Entry{
		Name:        "chart",
		Toolset:     "visualization",
		Description: "Create a chart, graph, or data visualization. Generate bar charts, line graphs, pie charts, area charts, scatter plots, and more to visualize data, statistics, trends, or results.",
		Emoji:       "📊",
		Schema: ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        "chart",
				Description: "Create an interactive chart or graph to visualize data",
				Parameters: json.RawMessage(`{
  "type": "object",
  "properties": {
    "type": {
      "type": "string",
      "enum": ["area", "bar", "line", "composed", "scatter", "pie", "radar", "radialBar", "treemap", "funnel"],
      "description": "The type of chart to be generated."
    },
    "title": {
      "type": "string",
      "description": "Title of the chart. There MUST always be a title. Do not leave it blank."
    },
    "dataset": {
      "type": "string",
      "description": "Valid JSON array where each element is an object with 'name' field and numeric fields for values. Format: [{\"name\":\"label\",\"metric\":value},...]. Provide JSON data only as a string."
    },
    "caption": {
      "type": "string",
      "description": "Optional notes or caption to display below the chart."
    }
  },
  "required": ["type", "title", "dataset"],
  "additionalProperties": false
}`),
			},
		},
		Handler:        HandleChart,
		MaxResultChars: 10000,
	})
}
