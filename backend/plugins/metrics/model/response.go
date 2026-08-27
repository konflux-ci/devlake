/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package model defines the FORMAT.md response types shared between the api
// and transform packages.  Keeping them here avoids the import cycle:
//
//	api → transform/pr → api   (was circular)
//	api → transform/pr → model (no cycle)
package model

// MetricResponse is the top-level response shape for every metrics endpoint.
// The dashboard reads Type to select a renderer and passes Data (plus Options
// when present) to Chart.js or a custom renderer.
type MetricResponse struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// LastUpdate is a Unix timestamp in milliseconds.
	LastUpdate int64       `json:"lastupdate"`
	Data       interface{} `json:"data"`
	Options    interface{} `json:"options,omitempty"`
	Summary    interface{} `json:"summary,omitempty"`
	Warning    string      `json:"warning,omitempty"`
}

// --- stats ---

// StatsData is the payload for type "stats".
type StatsData struct {
	Metrics []StatMetric `json:"metrics"`
	Extra   *ExtraData   `json:"extra,omitempty"`
}

// StatMetric is a single key-metric card.
type StatMetric struct {
	Label         string      `json:"label"`
	Value         interface{} `json:"value"`
	Goal          interface{} `json:"goal,omitempty"`
	Informational bool        `json:"informational,omitempty"`
	Explain       string      `json:"explain,omitempty"`
}

// --- doughnut ---

// DoughnutData is the payload for type "doughnut".
type DoughnutData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
	Extra    *ExtraData     `json:"extra,omitempty"`
}

// --- line ---

// LineData is the payload for type "line".
type LineData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
}

// --- bar ---

// BarData is the payload for type "bar".
type BarData struct {
	Labels   []string       `json:"labels"`
	Datasets []ChartDataset `json:"datasets"`
	Extra    *ExtraData     `json:"extra,omitempty"`
}

// --- sankey ---

// SankeyData is the payload for type "sankey".
type SankeyData struct {
	Datasets             []SankeyDataset        `json:"datasets"`
	Extra                *ExtraData             `json:"extra,omitempty"`
	MedianReviewInterval float64                `json:"medianReviewInterval,omitempty"`
	Transfers            map[string]interface{} `json:"transfers,omitempty"`
	Percentages          map[string]interface{} `json:"percentages,omitempty"`
}

// SankeyDataset holds one flow diagram.
type SankeyDataset struct {
	Label     string            `json:"label"`
	Data      []SankeyFlow      `json:"data"`
	Color     map[string]string `json:"color,omitempty"`
	ColorMode string            `json:"colorMode,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Priority  map[string]int    `json:"priority,omitempty"`
	Column    map[string]int    `json:"column,omitempty"`
	Size      string            `json:"size,omitempty"`
}

// SankeyFlow is a single flow band between two nodes.
type SankeyFlow struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Flow  float64 `json:"flow"`
	Count int     `json:"count"`
}

// --- shared ---

// ChartDataset is a Chart.js dataset used by doughnut, line, and bar types.
type ChartDataset struct {
	Label           string      `json:"label,omitempty"`
	Data            interface{} `json:"data"`
	BackgroundColor interface{} `json:"backgroundColor,omitempty"`
	BorderColor     interface{} `json:"borderColor,omitempty"`
	BorderWidth     int         `json:"borderWidth,omitempty"`
	Extra           *ExtraData  `json:"extra,omitempty"`
}

// --- extra / drill-down ---

// ExtraData enables interactive drill-down lists from visualizations.
type ExtraData struct {
	Config []ExtraConfig          `json:"config"`
	Data   map[string]interface{} `json:"data"`
}

// ExtraConfig describes one drill-down link or interactive element.
type ExtraConfig struct {
	Location string                  `json:"location"`
	Type     string                  `json:"type"`
	Field    string                  `json:"field,omitempty"`
	Template string                  `json:"template,omitempty"`
	Labels   []ExtraLabel            `json:"labels,omitempty"`
	Elements map[string]ExtraElement `json:"elements,omitempty"`
}

// ExtraLabel defines a column in a drill-down popup table.
type ExtraLabel struct {
	Label string `json:"label"`
	Field string `json:"field"`
}

// ExtraElement maps a diagram element key to a data list.
type ExtraElement struct {
	Field    string `json:"field"`
	Template string `json:"template"`
}
