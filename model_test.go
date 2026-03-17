package orca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Classification Model Tests ---

func TestOpenClassificationModel(t *testing.T) {
	// Given
	meta := ClassificationModelMetadata{
		ID:         "cm-123",
		Name:       "test-classifier",
		NumClasses: 3,
		HeadType:   "RAC",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/classification_model/test-classifier" {
			json.NewEncoder(w).Encode(meta)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	model, err := c.OpenClassificationModel(context.Background(), "test-classifier")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Metadata.ID != "cm-123" {
		t.Errorf("expected ID=cm-123, got %s", model.Metadata.ID)
	}
	if model.Metadata.NumClasses != 3 {
		t.Errorf("expected NumClasses=3, got %d", model.Metadata.NumClasses)
	}
}

func TestOpenClassificationModelNotFound(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"detail": "model not found"}`))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.OpenClassificationModel(context.Background(), "nonexistent")

	// Then
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestClassificationPredict(t *testing.T) {
	// Given
	label := 1
	labelName := "positive"
	preds := []ClassificationPrediction{
		{Label: &label, LabelName: &labelName, Confidence: 0.95, Logits: []float64{0.1, 0.9}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/classification_model/cm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(ClassificationModelMetadata{ID: "cm-123", Name: "test"})
		case r.URL.Path == "/gpu/classification_model/cm-123/prediction" && r.Method == "POST":
			json.NewEncoder(w).Encode(preds)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenClassificationModel(context.Background(), "cm-123")

	// When
	pred, err := model.Predict(context.Background(), "this is great")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *pred.Label != 1 {
		t.Errorf("expected label=1, got %d", *pred.Label)
	}
	if *pred.LabelName != "positive" {
		t.Errorf("expected label_name=positive, got %s", *pred.LabelName)
	}
	if pred.Confidence != 0.95 {
		t.Errorf("expected confidence=0.95, got %f", pred.Confidence)
	}
	if len(pred.Logits) != 2 {
		t.Errorf("expected 2 logits, got %d", len(pred.Logits))
	}
}

func TestClassificationPredictBatch(t *testing.T) {
	// Given
	label1, label2 := 1, 0
	preds := []ClassificationPrediction{
		{Label: &label1, Confidence: 0.95},
		{Label: &label2, Confidence: 0.80},
	}
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/classification_model/cm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(ClassificationModelMetadata{ID: "cm-123", Name: "test"})
		case r.URL.Path == "/gpu/classification_model/cm-123/prediction" && r.Method == "POST":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(preds)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenClassificationModel(context.Background(), "cm-123")

	// When
	results, err := model.PredictBatch(context.Background(), []string{"good", "bad"},
		WithClassifySaveTelemetry(false),
		WithClassifyTags("test-tag"),
	)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(results))
	}
	if receivedBody["save_telemetry"].(bool) != false {
		t.Error("expected save_telemetry=false")
	}
	inputValues := receivedBody["input_values"].([]any)
	if len(inputValues) != 2 {
		t.Errorf("expected 2 input_values, got %d", len(inputValues))
	}
}

func TestClassificationPredictWithOptions(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/classification_model/cm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(ClassificationModelMetadata{ID: "cm-123", Name: "test"})
		case r.URL.Path == "/gpu/classification_model/cm-123/prediction" && r.Method == "POST":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			label := 0
			json.NewEncoder(w).Encode([]ClassificationPrediction{{Label: &label, Confidence: 0.5}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenClassificationModel(context.Background(), "cm-123")

	// When
	_, err := model.Predict(context.Background(), "test",
		WithExpectedLabels([]int{1}),
		WithClassifyPrompt("classify sentiment"),
		WithClassifyIgnoreUnlabeled(true),
		WithClassifyConsistencyLevel("Strong"),
		WithClassifyFilters(NewFilter("label", "!=", 0)),
	)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["prompt"].(string) != "classify sentiment" {
		t.Errorf("expected prompt in request")
	}
	if receivedBody["ignore_unlabeled"].(bool) != true {
		t.Error("expected ignore_unlabeled=true")
	}
	if receivedBody["consistency_level"].(string) != "Strong" {
		t.Error("expected consistency_level=Strong")
	}
}

func TestListClassificationModels(t *testing.T) {
	// Given
	models := []ClassificationModelMetadata{
		{ID: "cm-1", Name: "model-a", NumClasses: 2},
		{ID: "cm-2", Name: "model-b", NumClasses: 5},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/classification_model" {
			json.NewEncoder(w).Encode(models)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	result, err := c.ListClassificationModels(context.Background())

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 models, got %d", len(result))
	}
}

// --- Regression Model Tests ---

func TestOpenRegressionModel(t *testing.T) {
	// Given
	meta := RegressionModelMetadata{
		ID:       "rm-123",
		Name:     "test-regressor",
		HeadType: "RAR",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/regression_model/test-regressor" {
			json.NewEncoder(w).Encode(meta)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	model, err := c.OpenRegressionModel(context.Background(), "test-regressor")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Metadata.ID != "rm-123" {
		t.Errorf("expected ID=rm-123, got %s", model.Metadata.ID)
	}
}

func TestRegressionPredict(t *testing.T) {
	// Given
	score := 0.85
	preds := []RegressionPrediction{
		{Score: &score, Confidence: 0.90},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/regression_model/rm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(RegressionModelMetadata{ID: "rm-123", Name: "test"})
		case r.URL.Path == "/gpu/regression_model/rm-123/prediction" && r.Method == "POST":
			json.NewEncoder(w).Encode(preds)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenRegressionModel(context.Background(), "rm-123")

	// When
	pred, err := model.Predict(context.Background(), "test input")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *pred.Score != 0.85 {
		t.Errorf("expected score=0.85, got %f", *pred.Score)
	}
	if pred.Confidence != 0.90 {
		t.Errorf("expected confidence=0.90, got %f", pred.Confidence)
	}
}

func TestRegressionPredictBatch(t *testing.T) {
	// Given
	score1, score2 := 0.1, 0.9
	preds := []RegressionPrediction{
		{Score: &score1, Confidence: 0.80},
		{Score: &score2, Confidence: 0.95},
	}
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/regression_model/rm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(RegressionModelMetadata{ID: "rm-123", Name: "test"})
		case r.URL.Path == "/gpu/regression_model/rm-123/prediction" && r.Method == "POST":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(preds)
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenRegressionModel(context.Background(), "rm-123")

	// When
	results, err := model.PredictBatch(context.Background(), []string{"a", "b"},
		WithExpectedScores([]float64{0.0, 1.0}),
		WithRegressSaveTelemetry(false),
	)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(results))
	}
	if *results[0].Score != 0.1 {
		t.Errorf("expected first score=0.1, got %f", *results[0].Score)
	}
	if receivedBody["save_telemetry"].(bool) != false {
		t.Error("expected save_telemetry=false")
	}
}

func TestRegressionPredictWithOptions(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/regression_model/rm-123" && r.Method == "GET":
			json.NewEncoder(w).Encode(RegressionModelMetadata{ID: "rm-123", Name: "test"})
		case r.URL.Path == "/gpu/regression_model/rm-123/prediction" && r.Method == "POST":
			json.NewDecoder(r.Body).Decode(&receivedBody)
			score := 0.5
			json.NewEncoder(w).Encode([]RegressionPrediction{{Score: &score, Confidence: 0.5}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, _ := c.OpenRegressionModel(context.Background(), "rm-123")

	// When
	_, err := model.Predict(context.Background(), "test",
		WithRegressPrompt("score sentiment"),
		WithRegressIgnoreUnlabeled(true),
		WithRegressConsistencyLevel("Strong"),
		WithRegressTags("experiment"),
	)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody["prompt"].(string) != "score sentiment" {
		t.Errorf("expected prompt in request")
	}
	if receivedBody["consistency_level"].(string) != "Strong" {
		t.Error("expected consistency_level=Strong")
	}
}

func TestListRegressionModels(t *testing.T) {
	// Given
	models := []RegressionModelMetadata{
		{ID: "rm-1", Name: "regressor-a"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/regression_model" {
			json.NewEncoder(w).Encode(models)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	result, err := c.ListRegressionModels(context.Background())

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 model, got %d", len(result))
	}
	if result[0].Name != "regressor-a" {
		t.Errorf("expected name=regressor-a, got %s", result[0].Name)
	}
}
