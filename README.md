# Orca Go SDK

A minimal Go client for the [Orca](https://orcadb.ai) API. Zero external dependencies beyond the Go standard library.

## Installation

```bash
GONOSUMDB=github.com/orcadb/orca-go GONOSUMCHECK=github.com/orcadb/orca-go GOPROXY=direct go get github.com/orcadb/orca-go@latest
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    orca "github.com/orcadb/orca-go"
)

func main() {
    ctx := context.Background()

    // Reads ORCA_API_KEY and ORCA_API_URL from environment variables
    client := orca.NewClient()

    // Or configure explicitly
    client = orca.NewClient(
        orca.WithAPIKey("your-api-key"),
        orca.WithBaseURL("https://api.orcadb.ai/"),
    )

    // Check connectivity
    if !client.IsHealthy(ctx) {
        log.Fatal("API is not reachable")
    }

    // Open a memoryset by name or ID
    ms, err := client.OpenMemoryset(ctx, "my-memoryset")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Memoryset %s has %d memories\n", ms.Metadata.Name, ms.Count())

    // Query memories with filters
    memories, err := ms.Query(ctx,
        orca.WithLimit(10),
        orca.WithFilters(orca.NewFilter("label", "==", 1)),
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, m := range memories {
        fmt.Println(m.Value)
    }

    // Search by semantic similarity
    results, err := ms.Search(ctx, "example query", orca.WithSearchCount(5))
    if err != nil {
        log.Fatal(err)
    }
    for _, r := range results {
        fmt.Printf("  %s (score: %.3f)\n", r.Value, r.LookupScore)
    }

    // Classification prediction
    model, err := client.OpenClassificationModel(ctx, "my-model")
    if err != nil {
        log.Fatal(err)
    }
    pred, err := model.Predict(ctx, "classify this text")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Label: %d, Confidence: %.3f\n", *pred.Label, pred.Confidence)
}
```

## Configuration

| Option | Environment Variable | Default |
|--------|---------------------|---------|
| `WithAPIKey(key)` | `ORCA_API_KEY` | *(none)* |
| `WithBaseURL(url)` | `ORCA_API_URL` | `https://api.orcadb.ai/` |
| `WithRetries(n)` | -- | `3` |
| `WithHTTPClient(c)` | -- | 30s timeout |

Options take precedence over environment variables.

## API Reference

### Client

```go
client := orca.NewClient(opts...)

client.IsHealthy(ctx)                                  // bool
client.OpenMemoryset(ctx, nameOrID)                    // (*Memoryset, error)
client.ListMemorysets(ctx)                             // ([]MemorysetMetadata, error)
client.OpenClassificationModel(ctx, nameOrID)          // (*ClassificationModel, error)
client.ListClassificationModels(ctx)                   // ([]ClassificationModelMetadata, error)
client.OpenRegressionModel(ctx, nameOrID)              // (*RegressionModel, error)
client.ListRegressionModels(ctx)                       // ([]RegressionModelMetadata, error)
```

### Memoryset

```go
ms.Count()                                  // int (from cached metadata)
ms.Refresh(ctx)                             // error (re-fetch metadata)
ms.Query(ctx, opts...)                      // ([]Memory, error)
ms.Search(ctx, query, opts...)              // ([]MemoryLookup, error)
ms.SearchBatch(ctx, queries, opts...)       // ([][]MemoryLookup, error)
ms.Get(ctx, memoryIDs...)                   // ([]Memory, error)
ms.Insert(ctx, items)                       // ([]string, error)
ms.Update(ctx, updates)                     // (int, error)
ms.UpdateByFilter(ctx, filters, patch)      // (int, error)
ms.Delete(ctx, memoryIDs...)                // (int, error)
ms.DeleteByFilter(ctx, filters)             // (int, error)
ms.Truncate(ctx)                            // (int, error)
```

**Query options:** `WithOffset(n)`, `WithLimit(n)`, `WithFilters(...)`, `WithConsistencyLevel(level)`

**Search options:** `WithSearchCount(n)`, `WithSearchPrompt(p)`, `WithSearchPartitionID(id)`, `WithSearchConsistencyLevel(level)`

### Classification Model

```go
model.Predict(ctx, value, opts...)          // (*ClassificationPrediction, error)
model.PredictBatch(ctx, values, opts...)    // ([]ClassificationPrediction, error)
```

**Options:** `WithExpectedLabels(labels)`, `WithClassifyFilters(...)`, `WithClassifyTags(...)`, `WithClassifySaveTelemetry(bool)`, `WithClassifyPrompt(p)`, `WithClassifyIgnoreUnlabeled(bool)`, `WithClassifyConsistencyLevel(level)`

### Regression Model

```go
model.Predict(ctx, value, opts...)          // (*RegressionPrediction, error)
model.PredictBatch(ctx, values, opts...)    // ([]RegressionPrediction, error)
```

**Options:** `WithExpectedScores(scores)`, `WithRegressTags(...)`, `WithRegressSaveTelemetry(bool)`, `WithRegressPrompt(p)`, `WithRegressIgnoreUnlabeled(bool)`, `WithRegressConsistencyLevel(level)`

### Filters

```go
orca.NewFilter("label", "==", 1)
orca.NewFilter("metadata.category", "in", []string{"a", "b"})
orca.NewFilter("source_id", "!=", "excluded")

// Use in queries
ms.Query(ctx, orca.WithFilters(
    orca.NewFilter("label", "==", 1),
    orca.NewFilter("metadata.status", "==", "active"),
))

// Use in delete
ms.DeleteByFilter(ctx, []orca.Filter{
    orca.NewFilter("source_id", "==", "old-batch"),
})

// Use in update
newLabel := 2
ms.UpdateByFilter(ctx,
    []orca.Filter{orca.NewFilter("label", "==", 1)},
    orca.MemoryPatch{Label: &newLabel},
)
```

### Error Handling

```go
ms, err := client.OpenMemoryset(ctx, "nonexistent")
if orca.IsNotFound(err) {
    // memoryset doesn't exist
}
if orca.IsUnauthorized(err) {
    // invalid API key
}
if orca.IsForbidden(err) {
    // insufficient permissions
}
if orca.IsValidationError(err) {
    // invalid request parameters
}

// Access the full API error
var apiErr *orca.APIError
if errors.As(err, &apiErr) {
    fmt.Printf("Status %d: %s\n", apiErr.StatusCode, apiErr.Message)
}
```

### Inserting Memories

```go
label := 1
ids, err := ms.Insert(ctx, []orca.MemoryInsert{
    {Value: "great product", Label: &label},
    {Value: "loved it", Label: &label},
})
```

For scored memorysets, use `Score` instead of `Label`:

```go
score := 4.5
ids, err := ms.Insert(ctx, []orca.MemoryInsert{
    {Value: "great product", Score: &score},
})
```
