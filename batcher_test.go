package orca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- helpers ---

func newClassifyBatcherServer(t *testing.T, handler http.HandlerFunc) (*ClassificationModel, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/classification_model/cm-1" && r.Method == "GET":
			json.NewEncoder(w).Encode(ClassificationModelMetadata{ID: "cm-1", Name: "test"})
		case r.URL.Path == "/gpu/classification_model/cm-1/prediction" && r.Method == "POST":
			handler(w, r)
		default:
			w.WriteHeader(404)
		}
	}))
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, err := c.OpenClassificationModel(context.Background(), "cm-1")
	if err != nil {
		srv.Close()
		t.Fatalf("opening model: %v", err)
	}
	return model, srv.Close
}

func newRegressBatcherServer(t *testing.T, handler http.HandlerFunc) (*RegressionModel, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/regression_model/rm-1" && r.Method == "GET":
			json.NewEncoder(w).Encode(RegressionModelMetadata{ID: "rm-1", Name: "test"})
		case r.URL.Path == "/gpu/regression_model/rm-1/prediction" && r.Method == "POST":
			handler(w, r)
		default:
			w.WriteHeader(404)
		}
	}))
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	model, err := c.OpenRegressionModel(context.Background(), "rm-1")
	if err != nil {
		srv.Close()
		t.Fatalf("opening model: %v", err)
	}
	return model, srv.Close
}

func echoClassifyHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InputValues []string `json:"input_values"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	preds := make([]ClassificationPrediction, len(body.InputValues))
	for i := range body.InputValues {
		label := i
		preds[i] = ClassificationPrediction{Label: &label, Confidence: float64(i) * 0.1}
	}
	json.NewEncoder(w).Encode(preds)
}

func echoRegressHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InputValues []string `json:"input_values"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	preds := make([]RegressionPrediction, len(body.InputValues))
	for i := range body.InputValues {
		score := float64(i) * 0.1
		preds[i] = RegressionPrediction{Score: &score, Confidence: 0.9}
	}
	json.NewEncoder(w).Encode(preds)
}

// --- Classification Batcher Tests ---

func TestClassificationBatcher_SinglePredict(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, echoClassifyHandler)
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(10),
		WithBatchDelay(50 * time.Millisecond),
	})
	defer batcher.Close()

	pred, err := batcher.Predict(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pred.Label == nil || *pred.Label != 0 {
		t.Errorf("expected label=0, got %v", pred.Label)
	}
}

func TestClassificationBatcher_FlushOnBatchSize(t *testing.T) {
	var batchCalls atomic.Int32
	var mu sync.Mutex
	var batchSizes []int

	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InputValues []string `json:"input_values"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		batchCalls.Add(1)
		mu.Lock()
		batchSizes = append(batchSizes, len(body.InputValues))
		mu.Unlock()

		preds := make([]ClassificationPrediction, len(body.InputValues))
		for i := range body.InputValues {
			label := i
			preds[i] = ClassificationPrediction{Label: &label, Confidence: 0.9}
		}
		json.NewEncoder(w).Encode(preds)
	})
	defer cleanup()

	const batchSize = 3
	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(batchSize),
		WithBatchDelay(10 * time.Second), // large delay so only size triggers flush
	})
	defer batcher.Close()

	var wg sync.WaitGroup
	results := make([]*ClassificationPrediction, batchSize)
	errs := make([]error, batchSize)
	for i := 0; i < batchSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = batcher.Predict(context.Background(), "value")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("prediction %d: unexpected error: %v", i, err)
		}
	}

	if calls := batchCalls.Load(); calls != 1 {
		t.Errorf("expected 1 batch call, got %d", calls)
	}
	mu.Lock()
	if len(batchSizes) != 1 || batchSizes[0] != batchSize {
		t.Errorf("expected batch of size %d, got %v", batchSize, batchSizes)
	}
	mu.Unlock()
}

func TestClassificationBatcher_FlushOnDelay(t *testing.T) {
	var batchCalls atomic.Int32

	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InputValues []string `json:"input_values"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		batchCalls.Add(1)

		preds := make([]ClassificationPrediction, len(body.InputValues))
		for i := range body.InputValues {
			label := i
			preds[i] = ClassificationPrediction{Label: &label, Confidence: 0.9}
		}
		json.NewEncoder(w).Encode(preds)
	})
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(100), // large batch size so only delay triggers flush
		WithBatchDelay(50 * time.Millisecond),
	})
	defer batcher.Close()

	pred, err := batcher.Predict(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pred == nil {
		t.Fatal("expected non-nil prediction")
	}
	if calls := batchCalls.Load(); calls != 1 {
		t.Errorf("expected 1 batch call, got %d", calls)
	}
}

func TestClassificationBatcher_MultipleBatches(t *testing.T) {
	var batchCalls atomic.Int32

	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InputValues []string `json:"input_values"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		batchCalls.Add(1)

		preds := make([]ClassificationPrediction, len(body.InputValues))
		for i := range body.InputValues {
			label := i
			preds[i] = ClassificationPrediction{Label: &label, Confidence: 0.9}
		}
		json.NewEncoder(w).Encode(preds)
	})
	defer cleanup()

	const batchSize = 2
	const totalItems = 5
	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(batchSize),
		WithBatchDelay(50 * time.Millisecond),
	})

	var wg sync.WaitGroup
	errs := make([]error, totalItems)
	for i := 0; i < totalItems; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = batcher.Predict(context.Background(), "value")
		}(i)
	}
	wg.Wait()
	batcher.Close()

	for i, err := range errs {
		if err != nil {
			t.Errorf("prediction %d: unexpected error: %v", i, err)
		}
	}

	// 5 items with batch size 2: at least 2 batch calls (2+2+1), possibly 3
	if calls := batchCalls.Load(); calls < 2 {
		t.Errorf("expected at least 2 batch calls, got %d", calls)
	}
}

func TestClassificationBatcher_CloseFlushesRemaining(t *testing.T) {
	var batchCalls atomic.Int32

	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InputValues []string `json:"input_values"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		batchCalls.Add(1)

		preds := make([]ClassificationPrediction, len(body.InputValues))
		for i := range body.InputValues {
			label := i
			preds[i] = ClassificationPrediction{Label: &label, Confidence: 0.9}
		}
		json.NewEncoder(w).Encode(preds)
	})
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(100),
		WithBatchDelay(10 * time.Second),
	})

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = batcher.Predict(context.Background(), "value")
		}(i)
	}

	// Give goroutines time to submit, then close to trigger flush.
	time.Sleep(50 * time.Millisecond)
	batcher.Close()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("prediction %d: unexpected error: %v", i, err)
		}
	}
	if calls := batchCalls.Load(); calls < 1 {
		t.Errorf("expected at least 1 batch call from close flush, got %d", calls)
	}
}

func TestClassificationBatcher_ErrorPropagation(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"detail":"internal error"}`))
	})
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(2),
		WithBatchDelay(50 * time.Millisecond),
	})
	defer batcher.Close()

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = batcher.Predict(context.Background(), "value")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("prediction %d: expected error, got nil", i)
		}
	}
}

func TestClassificationBatcher_ContextCancellation(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		echoClassifyHandler(w, r)
	})
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(1),
		WithBatchDelay(20 * time.Millisecond),
	})
	defer batcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := batcher.Predict(ctx, "hello")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestClassificationBatcher_PredictAfterClose(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, echoClassifyHandler)
	defer cleanup()

	batcher := model.NewBatcher(nil)
	batcher.Close()

	_, err := batcher.Predict(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error from closed batcher")
	}
}

func TestClassificationBatcher_DoubleClose(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, echoClassifyHandler)
	defer cleanup()

	batcher := model.NewBatcher(nil)
	batcher.Close()
	batcher.Close() // should not panic
}

func TestClassificationBatcher_WithClassifyOptions(t *testing.T) {
	var receivedPrompt string

	model, cleanup := newClassifyBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if p, ok := body["prompt"].(string); ok {
			receivedPrompt = p
		}
		label := 0
		json.NewEncoder(w).Encode([]ClassificationPrediction{{Label: &label, Confidence: 0.5}})
	})
	defer cleanup()

	batcher := model.NewBatcher(
		[]BatcherOption{WithBatchDelay(50 * time.Millisecond)},
		WithClassifyPrompt("sentiment analysis"),
	)
	defer batcher.Close()

	_, err := batcher.Predict(context.Background(), "great movie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPrompt != "sentiment analysis" {
		t.Errorf("expected prompt 'sentiment analysis', got %q", receivedPrompt)
	}
}

func TestClassificationBatcher_DefaultConfig(t *testing.T) {
	model, cleanup := newClassifyBatcherServer(t, echoClassifyHandler)
	defer cleanup()

	batcher := model.NewBatcher(nil)
	defer batcher.Close()

	if batcher.cfg.maxBatch != DefaultBatchSize {
		t.Errorf("expected default batch size %d, got %d", DefaultBatchSize, batcher.cfg.maxBatch)
	}
	if batcher.cfg.maxDelay != DefaultBatchDelay {
		t.Errorf("expected default batch delay %v, got %v", DefaultBatchDelay, batcher.cfg.maxDelay)
	}
}

// --- Regression Batcher Tests ---

func TestRegressionBatcher_SinglePredict(t *testing.T) {
	model, cleanup := newRegressBatcherServer(t, echoRegressHandler)
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(10),
		WithBatchDelay(50 * time.Millisecond),
	})
	defer batcher.Close()

	pred, err := batcher.Predict(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pred.Score == nil {
		t.Fatal("expected non-nil score")
	}
}

func TestRegressionBatcher_FlushOnBatchSize(t *testing.T) {
	var batchCalls atomic.Int32

	model, cleanup := newRegressBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			InputValues []string `json:"input_values"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		batchCalls.Add(1)

		preds := make([]RegressionPrediction, len(body.InputValues))
		for i := range body.InputValues {
			score := float64(i)
			preds[i] = RegressionPrediction{Score: &score, Confidence: 0.9}
		}
		json.NewEncoder(w).Encode(preds)
	})
	defer cleanup()

	const batchSize = 3
	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(batchSize),
		WithBatchDelay(10 * time.Second),
	})
	defer batcher.Close()

	var wg sync.WaitGroup
	errs := make([]error, batchSize)
	for i := 0; i < batchSize; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = batcher.Predict(context.Background(), "value")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("prediction %d: unexpected error: %v", i, err)
		}
	}
	if calls := batchCalls.Load(); calls != 1 {
		t.Errorf("expected 1 batch call, got %d", calls)
	}
}

func TestRegressionBatcher_FlushOnDelay(t *testing.T) {
	model, cleanup := newRegressBatcherServer(t, echoRegressHandler)
	defer cleanup()

	batcher := model.NewBatcher([]BatcherOption{
		WithBatchSize(100),
		WithBatchDelay(50 * time.Millisecond),
	})
	defer batcher.Close()

	pred, err := batcher.Predict(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pred == nil {
		t.Fatal("expected non-nil prediction")
	}
}

func TestRegressionBatcher_WithRegressOptions(t *testing.T) {
	var receivedPrompt string

	model, cleanup := newRegressBatcherServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if p, ok := body["prompt"].(string); ok {
			receivedPrompt = p
		}
		score := 0.5
		json.NewEncoder(w).Encode([]RegressionPrediction{{Score: &score, Confidence: 0.5}})
	})
	defer cleanup()

	batcher := model.NewBatcher(
		[]BatcherOption{WithBatchDelay(50 * time.Millisecond)},
		WithRegressPrompt("score quality"),
	)
	defer batcher.Close()

	_, err := batcher.Predict(context.Background(), "great movie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedPrompt != "score quality" {
		t.Errorf("expected prompt 'score quality', got %q", receivedPrompt)
	}
}

func TestRegressionBatcher_PredictAfterClose(t *testing.T) {
	model, cleanup := newRegressBatcherServer(t, echoRegressHandler)
	defer cleanup()

	batcher := model.NewBatcher(nil)
	batcher.Close()

	_, err := batcher.Predict(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error from closed batcher")
	}
}
