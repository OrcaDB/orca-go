package orca

import "strings"

// Memory represents a memory entry in a memoryset.
// For labeled memorysets, Label and LabelName are populated.
// For scored memorysets, Score is populated.
type Memory struct {
	MemoryID      string         `json:"memory_id"`
	Value         string         `json:"value"`
	Label         *int           `json:"label,omitempty"`
	LabelName     *string        `json:"label_name,omitempty"`
	Score         *float64       `json:"score,omitempty"`
	SourceID      *string        `json:"source_id"`
	PartitionID   *string        `json:"partition_id"`
	Metadata      map[string]any `json:"metadata"`
	Embedding     []float64      `json:"embedding,omitempty"`
	MemoryVersion int            `json:"memory_version"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	EditedAt      string         `json:"edited_at"`
}

// MemoryLookup is a Memory with an additional similarity score from a search.
type MemoryLookup struct {
	Memory
	LookupScore float64 `json:"lookup_score"`
}

// MemoryInsert describes a memory to insert into a memoryset.
// Set Label for labeled memorysets, Score for scored memorysets.
type MemoryInsert struct {
	Value       string         `json:"value"`
	Label       *int           `json:"label,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	SourceID    *string        `json:"source_id,omitempty"`
	PartitionID *string        `json:"partition_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	MemoryID    *string        `json:"memory_id,omitempty"`
}

// MemoryUpdate describes updates to a specific memory identified by MemoryID.
type MemoryUpdate struct {
	MemoryID    string         `json:"memory_id"`
	Value       *string        `json:"value,omitempty"`
	Label       *int           `json:"label,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	SourceID    *string        `json:"source_id,omitempty"`
	PartitionID *string        `json:"partition_id,omitempty"`
}

// MemoryPatch describes fields to update on all memories matching a filter.
type MemoryPatch struct {
	Value       *string        `json:"value,omitempty"`
	Label       *int           `json:"label,omitempty"`
	Score       *float64       `json:"score,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	SourceID    *string        `json:"source_id,omitempty"`
	PartitionID *string        `json:"partition_id,omitempty"`
}

// Filter is a query condition for filtering memories.
type Filter struct {
	Field     []string `json:"field"`
	Operation string   `json:"op"`
	Value     any      `json:"value"`
}

// NewFilter creates a Filter from a dot-separated field path, an operator, and a value.
//
//	NewFilter("label", "==", 0)
//	NewFilter("metadata.author", "like", "John")
//	NewFilter("source_id", "in", []string{"a", "b"})
func NewFilter(field, op string, value any) Filter {
	return Filter{
		Field:     strings.Split(field, "."),
		Operation: op,
		Value:     value,
	}
}

// MemorysetMetadata contains metadata about a memoryset.
type MemorysetMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	MemoryType  string   `json:"memory_type"`
	Length      int      `json:"length"`
	LabelNames  []string `json:"label_names"`
}

// ClassificationModelMetadata contains metadata about a classification model.
type ClassificationModelMetadata struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	MemorysetID       string  `json:"memoryset_id"`
	NumClasses        int     `json:"num_classes"`
	MemoryLookupCount int     `json:"memory_lookup_count"`
	HeadType          string  `json:"head_type"`
}

// RegressionModelMetadata contains metadata about a regression model.
type RegressionModelMetadata struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	Description       *string `json:"description"`
	MemorysetID       string  `json:"memoryset_id"`
	MemoryLookupCount int     `json:"memory_lookup_count"`
	HeadType          string  `json:"head_type"`
}

// ClassificationPrediction is the result of a classification model prediction.
type ClassificationPrediction struct {
	PredictionID *string   `json:"prediction_id"`
	Label        *int      `json:"label"`
	LabelName    *string   `json:"label_name"`
	Confidence   float64   `json:"confidence"`
	AnomalyScore *float64  `json:"anomaly_score"`
	Logits       []float64 `json:"logits"`
}

// RegressionPrediction is the result of a regression model prediction.
type RegressionPrediction struct {
	PredictionID *string  `json:"prediction_id"`
	Score        *float64 `json:"score"`
	Confidence   float64  `json:"confidence"`
	AnomalyScore *float64 `json:"anomaly_score"`
}
