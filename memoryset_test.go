package orca

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestMemorysetServer(meta MemorysetMetadata, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *Client) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/memoryset/"+meta.ID {
			json.NewEncoder(w).Encode(meta)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/memoryset/"+meta.Name {
			json.NewEncoder(w).Encode(meta)
			return
		}
		handler(w, r)
	}))
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	return srv, c
}

var testMeta = MemorysetMetadata{
	ID:         "ms-abc-123",
	Name:       "test-memoryset",
	MemoryType: "labeled",
	Length:     42,
	LabelNames: []string{"cat", "dog"},
}

func TestOpenMemorysetByName(t *testing.T) {
	// Given
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer srv.Close()

	// When
	ms, err := c.OpenMemoryset(context.Background(), "test-memoryset")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.Metadata.ID != "ms-abc-123" {
		t.Errorf("expected ID=ms-abc-123, got %s", ms.Metadata.ID)
	}
	if ms.Count() != 42 {
		t.Errorf("expected Count()=42, got %d", ms.Count())
	}
}

func TestOpenMemorysetByID(t *testing.T) {
	// Given
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	defer srv.Close()

	// When
	ms, err := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.Metadata.Name != "test-memoryset" {
		t.Errorf("expected Name=test-memoryset, got %s", ms.Metadata.Name)
	}
}

func TestOpenMemorysetNotFound(t *testing.T) {
	// Given
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"detail": "not found"}`))
	}))
	defer srv.Close()
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	_, err := c.OpenMemoryset(context.Background(), "nonexistent")

	// Then
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound error, got: %v", err)
	}
}

func TestListMemorysets(t *testing.T) {
	// Given
	metas := []MemorysetMetadata{
		{ID: "ms-1", Name: "first", MemoryType: "labeled", Length: 10},
		{ID: "ms-2", Name: "second", MemoryType: "scored", Length: 20},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/memoryset" {
			json.NewEncoder(w).Encode(metas)
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))

	// When
	result, err := c.ListMemorysets(context.Background())

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 memorysets, got %d", len(result))
	}
	if result[1].Name != "second" {
		t.Errorf("expected second memoryset name=second, got %s", result[1].Name)
	}
}

func TestMemorysetQuery(t *testing.T) {
	// Given
	memories := []Memory{
		{MemoryID: "m-1", Value: "hello"},
		{MemoryID: "m-2", Value: "world"},
	}
	var receivedBody map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(memories)
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	result, err := ms.Query(context.Background(), WithLimit(50), WithOffset(10))

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(result))
	}
	if result[0].Value != "hello" {
		t.Errorf("expected first value=hello, got %s", result[0].Value)
	}
	if receivedBody["limit"].(float64) != 50 {
		t.Errorf("expected limit=50, got %v", receivedBody["limit"])
	}
	if receivedBody["offset"].(float64) != 10 {
		t.Errorf("expected offset=10, got %v", receivedBody["offset"])
	}
}

func TestMemorysetQueryWithFilters(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode([]Memory{})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	_, err := ms.Query(context.Background(), WithFilters(
		NewFilter("label", "==", 1),
		NewFilter("metadata.author", "like", "John"),
	))

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filters := receivedBody["filters"].([]any)
	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}
	first := filters[0].(map[string]any)
	if first["op"].(string) != "==" {
		t.Errorf("expected first filter op==, got %s", first["op"])
	}
}

func TestMemorysetSearch(t *testing.T) {
	// Given
	results := [][]MemoryLookup{
		{
			{Memory: Memory{MemoryID: "m-1", Value: "hello"}, LookupScore: 0.95},
			{Memory: Memory{MemoryID: "m-2", Value: "hi"}, LookupScore: 0.80},
		},
	}
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gpu/memoryset/ms-abc-123/lookup" && r.Method == "POST" {
			json.NewEncoder(w).Encode(results)
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	lookups, err := ms.Search(context.Background(), "test query", WithSearchCount(5))

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lookups) != 2 {
		t.Fatalf("expected 2 lookups, got %d", len(lookups))
	}
	if lookups[0].LookupScore != 0.95 {
		t.Errorf("expected lookup_score=0.95, got %f", lookups[0].LookupScore)
	}
	if lookups[0].Value != "hello" {
		t.Errorf("expected value=hello, got %s", lookups[0].Value)
	}
}

func TestMemorysetSearchBatch(t *testing.T) {
	// Given
	results := [][]MemoryLookup{
		{{Memory: Memory{MemoryID: "m-1"}, LookupScore: 0.9}},
		{{Memory: Memory{MemoryID: "m-2"}, LookupScore: 0.8}},
	}
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gpu/memoryset/ms-abc-123/lookup" && r.Method == "POST" {
			json.NewEncoder(w).Encode(results)
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	batched, err := ms.SearchBatch(context.Background(), []string{"q1", "q2"}, WithSearchCount(1))

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(batched) != 2 {
		t.Fatalf("expected 2 result groups, got %d", len(batched))
	}
}

func TestMemorysetGet(t *testing.T) {
	// Given
	memories := []Memory{
		{MemoryID: "m-1", Value: "hello"},
		{MemoryID: "m-2", Value: "world"},
	}
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories/get" && r.Method == "POST" {
			json.NewEncoder(w).Encode(memories)
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	result, err := ms.Get(context.Background(), "m-1", "m-2")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(result))
	}
}

func TestMemorysetInsert(t *testing.T) {
	// Given
	var receivedItems []map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gpu/memoryset/ms-abc-123/memory" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedItems)
			json.NewEncoder(w).Encode([]string{"id-1", "id-2"})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	label := 1
	// When
	ids, err := ms.Insert(context.Background(), []MemoryInsert{
		{Value: "hello", Label: &label},
		{Value: "world", Label: &label},
	})

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(ids))
	}
	if len(receivedItems) != 2 {
		t.Fatalf("expected 2 items in request, got %d", len(receivedItems))
	}
	if receivedItems[0]["value"].(string) != "hello" {
		t.Errorf("expected first item value=hello, got %v", receivedItems[0]["value"])
	}
}

func TestMemorysetUpdate(t *testing.T) {
	// Given
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gpu/memoryset/ms-abc-123/memories" && r.Method == "PATCH" {
			json.NewEncoder(w).Encode(map[string]int{"updated_count": 2})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	newVal := "updated"
	// When
	count, err := ms.Update(context.Background(), []MemoryUpdate{
		{MemoryID: "m-1", Value: &newVal},
		{MemoryID: "m-2", Value: &newVal},
	})

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected updated_count=2, got %d", count)
	}
}

func TestMemorysetUpdateByFilter(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gpu/memoryset/ms-abc-123/memories" && r.Method == "PATCH" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(map[string]int{"updated_count": 5})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	newLabel := 2
	// When
	count, err := ms.UpdateByFilter(context.Background(),
		[]Filter{NewFilter("label", "==", 1)},
		MemoryPatch{Label: &newLabel},
	)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected updated_count=5, got %d", count)
	}
	if receivedBody["filters"] == nil {
		t.Error("expected filters in request body")
	}
	if receivedBody["patch"] == nil {
		t.Error("expected patch in request body")
	}
}

func TestMemorysetDelete(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories/delete" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(map[string]int{"deleted_count": 3})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	count, err := ms.Delete(context.Background(), "m-1", "m-2", "m-3")

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected deleted_count=3, got %d", count)
	}
	ids := receivedBody["memory_ids"].([]any)
	if len(ids) != 3 {
		t.Errorf("expected 3 memory_ids in request, got %d", len(ids))
	}
}

func TestMemorysetDeleteByFilter(t *testing.T) {
	// Given
	var receivedBody map[string]any
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories/delete" && r.Method == "POST" {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			json.NewEncoder(w).Encode(map[string]int{"deleted_count": 10})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	count, err := ms.DeleteByFilter(context.Background(), []Filter{
		NewFilter("label", "==", 0),
	})

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 10 {
		t.Errorf("expected deleted_count=10, got %d", count)
	}
}

func TestMemorysetTruncate(t *testing.T) {
	// Given
	srv, c := newTestMemorysetServer(testMeta, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/memoryset/ms-abc-123/memories/delete" && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]int{"deleted_count": 42})
			return
		}
		w.WriteHeader(404)
	})
	defer srv.Close()

	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	count, err := ms.Truncate(context.Background())

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 42 {
		t.Errorf("expected deleted_count=42, got %d", count)
	}
}

func TestMemorysetRefresh(t *testing.T) {
	// Given: metadata changes on server
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		meta := testMeta
		if callCount > 1 {
			meta.Length = 100
		}
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithAPIKey("test"), WithRetries(0))
	ms, _ := c.OpenMemoryset(context.Background(), "ms-abc-123")

	// When
	if ms.Count() != 42 {
		t.Fatalf("expected initial count=42, got %d", ms.Count())
	}
	err := ms.Refresh(context.Background())

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms.Count() != 100 {
		t.Errorf("expected refreshed count=100, got %d", ms.Count())
	}
}
