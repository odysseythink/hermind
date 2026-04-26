package tool

import (
	"errors"
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

// ValidateChartInput checks all chart parameters and returns an error if invalid.
// TODO: Implement full validation in Task 5 (integrate all validation checks)
func ValidateChartInput(input *ChartInput) error {
	return nil
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
// TODO: Implement in Task 4 (dataset JSON parsing and validation)
func ValidateDataset(datasetStr string) error {
	return nil
}

// DatasetSize returns the number of records in the dataset.
// TODO: Implement in Task 4 (dataset JSON parsing and validation)
func DatasetSize(datasetStr string) (int, error) {
	return 0, nil
}
