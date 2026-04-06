package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// --- Classification Model ---

// ClassificationModel is a handle to a remote classification model in Orca.
type ClassificationModel struct {
	client   *Client
	Metadata ClassificationModelMetadata
}

// OpenClassificationModel opens an existing classification model by name or ID.
func (c *Client) OpenClassificationModel(ctx context.Context, nameOrID string) (*ClassificationModel, error) {
	data, err := c.get(ctx, "/classification_model/"+url.PathEscape(nameOrID), nil)
	if err != nil {
		return nil, err
	}
	var meta ClassificationModelMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing classification model metadata: %w", err)
	}
	return &ClassificationModel{client: c, Metadata: meta}, nil
}

// ListClassificationModels lists all classification models visible to the current API key.
func (c *Client) ListClassificationModels(ctx context.Context) ([]ClassificationModelMetadata, error) {
	data, err := c.get(ctx, "/classification_model", nil)
	if err != nil {
		return nil, err
	}
	var models []ClassificationModelMetadata
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("parsing classification model list: %w", err)
	}
	return models, nil
}

// --- Classification predict options ---

type classifyParams struct {
	InputValues         []string `json:"input_values"`
	ExpectedLabels      []int    `json:"expected_labels,omitempty"`
	Filters             []Filter `json:"filters,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	SaveTelemetry       bool     `json:"save_telemetry"`
	Prompt              *string  `json:"prompt,omitempty"`
	UseLookupCache      bool     `json:"use_lookup_cache"`
	IgnoreUnlabeled     *bool    `json:"ignore_unlabeled,omitempty"`
	PartitionFilterMode string   `json:"partition_filter_mode"`
	ConsistencyLevel    string   `json:"consistency_level"`
}

// ClassifyOption configures a classification Predict or PredictBatch call.
type ClassifyOption func(*classifyParams)

// WithExpectedLabels sets expected labels for telemetry/evaluation.
func WithExpectedLabels(labels []int) ClassifyOption {
	return func(p *classifyParams) { p.ExpectedLabels = labels }
}

// WithClassifyFilters sets filter conditions applied to the memoryset during prediction.
func WithClassifyFilters(filters ...Filter) ClassifyOption {
	return func(p *classifyParams) { p.Filters = filters }
}

// WithClassifyTags sets tags for the prediction telemetry.
func WithClassifyTags(tags ...string) ClassifyOption {
	return func(p *classifyParams) { p.Tags = tags }
}

// WithClassifySaveTelemetry controls whether prediction telemetry is saved (default false).
func WithClassifySaveTelemetry(save bool) ClassifyOption {
	return func(p *classifyParams) { p.SaveTelemetry = save }
}

// WithClassifyPrompt sets an optional prompt to contextualize the prediction.
func WithClassifyPrompt(prompt string) ClassifyOption {
	return func(p *classifyParams) { p.Prompt = &prompt }
}

// WithClassifyIgnoreUnlabeled controls whether unlabeled memories are ignored.
func WithClassifyIgnoreUnlabeled(ignore bool) ClassifyOption {
	return func(p *classifyParams) { p.IgnoreUnlabeled = &ignore }
}

// WithClassifyConsistencyLevel sets the consistency level for the prediction.
func WithClassifyConsistencyLevel(level string) ClassifyOption {
	return func(p *classifyParams) { p.ConsistencyLevel = level }
}

// Predict makes a single classification prediction.
func (m *ClassificationModel) Predict(ctx context.Context, value string, opts ...ClassifyOption) (*ClassificationPrediction, error) {
	results, err := m.PredictBatch(ctx, []string{value}, opts...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty prediction response")
	}
	return &results[0], nil
}

// PredictBatch makes classification predictions for multiple input values.
func (m *ClassificationModel) PredictBatch(ctx context.Context, values []string, opts ...ClassifyOption) ([]ClassificationPrediction, error) {
	p := &classifyParams{
		InputValues:         values,
		SaveTelemetry:       false,
		UseLookupCache:      true,
		PartitionFilterMode: "include_global",
		ConsistencyLevel:    "Bounded",
	}
	for _, opt := range opts {
		opt(p)
	}

	data, err := m.client.post(ctx, "/gpu/classification_model/"+url.PathEscape(m.Metadata.ID)+"/prediction", p)
	if err != nil {
		return nil, err
	}
	var preds []ClassificationPrediction
	if err := json.Unmarshal(data, &preds); err != nil {
		return nil, fmt.Errorf("parsing classification predictions: %w", err)
	}
	return preds, nil
}

// --- Regression Model ---

// RegressionModel is a handle to a remote regression model in Orca.
type RegressionModel struct {
	client   *Client
	Metadata RegressionModelMetadata
}

// OpenRegressionModel opens an existing regression model by name or ID.
func (c *Client) OpenRegressionModel(ctx context.Context, nameOrID string) (*RegressionModel, error) {
	data, err := c.get(ctx, "/regression_model/"+url.PathEscape(nameOrID), nil)
	if err != nil {
		return nil, err
	}
	var meta RegressionModelMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing regression model metadata: %w", err)
	}
	return &RegressionModel{client: c, Metadata: meta}, nil
}

// ListRegressionModels lists all regression models visible to the current API key.
func (c *Client) ListRegressionModels(ctx context.Context) ([]RegressionModelMetadata, error) {
	data, err := c.get(ctx, "/regression_model", nil)
	if err != nil {
		return nil, err
	}
	var models []RegressionModelMetadata
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("parsing regression model list: %w", err)
	}
	return models, nil
}

// --- Regression predict options ---

type regressParams struct {
	InputValues         []string  `json:"input_values"`
	ExpectedScores      []float64 `json:"expected_scores,omitempty"`
	Tags                []string  `json:"tags,omitempty"`
	SaveTelemetry       bool      `json:"save_telemetry"`
	Prompt              *string   `json:"prompt,omitempty"`
	UseLookupCache      bool      `json:"use_lookup_cache"`
	IgnoreUnlabeled     *bool     `json:"ignore_unlabeled,omitempty"`
	PartitionFilterMode string    `json:"partition_filter_mode"`
	ConsistencyLevel    string    `json:"consistency_level"`
}

// RegressOption configures a regression Predict or PredictBatch call.
type RegressOption func(*regressParams)

// WithExpectedScores sets expected scores for telemetry/evaluation.
func WithExpectedScores(scores []float64) RegressOption {
	return func(p *regressParams) { p.ExpectedScores = scores }
}

// WithRegressTags sets tags for the prediction telemetry.
func WithRegressTags(tags ...string) RegressOption {
	return func(p *regressParams) { p.Tags = tags }
}

// WithRegressSaveTelemetry controls whether prediction telemetry is saved (default false).
func WithRegressSaveTelemetry(save bool) RegressOption {
	return func(p *regressParams) { p.SaveTelemetry = save }
}

// WithRegressPrompt sets an optional prompt to contextualize the prediction.
func WithRegressPrompt(prompt string) RegressOption {
	return func(p *regressParams) { p.Prompt = &prompt }
}

// WithRegressIgnoreUnlabeled controls whether unlabeled memories are ignored.
func WithRegressIgnoreUnlabeled(ignore bool) RegressOption {
	return func(p *regressParams) { p.IgnoreUnlabeled = &ignore }
}

// WithRegressConsistencyLevel sets the consistency level for the prediction.
func WithRegressConsistencyLevel(level string) RegressOption {
	return func(p *regressParams) { p.ConsistencyLevel = level }
}

// Predict makes a single regression prediction.
func (m *RegressionModel) Predict(ctx context.Context, value string, opts ...RegressOption) (*RegressionPrediction, error) {
	results, err := m.PredictBatch(ctx, []string{value}, opts...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty prediction response")
	}
	return &results[0], nil
}

// PredictBatch makes regression predictions for multiple input values.
func (m *RegressionModel) PredictBatch(ctx context.Context, values []string, opts ...RegressOption) ([]RegressionPrediction, error) {
	p := &regressParams{
		InputValues:         values,
		SaveTelemetry:       false,
		UseLookupCache:      true,
		PartitionFilterMode: "include_global",
		ConsistencyLevel:    "Bounded",
	}
	for _, opt := range opts {
		opt(p)
	}

	data, err := m.client.post(ctx, "/gpu/regression_model/"+url.PathEscape(m.Metadata.ID)+"/prediction", p)
	if err != nil {
		return nil, err
	}
	var preds []RegressionPrediction
	if err := json.Unmarshal(data, &preds); err != nil {
		return nil, fmt.Errorf("parsing regression predictions: %w", err)
	}
	return preds, nil
}
