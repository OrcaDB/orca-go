package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	orca "github.com/orcadb/orca-go"
)

func main() {
	ctx := context.Background()

	// Reads ORCA_API_KEY and ORCA_API_URL from environment variables
	client := orca.NewClient()

	// Or configure explicitly
	// client := orca.NewClient(
	//     orca.WithAPIKey("MY_API_KEY"),
	//     orca.WithBaseURL("https://api.orcadb.ai/"),
	// )

	// Check connectivity
	if !client.IsHealthy(ctx) {
		log.Fatal("API is not reachable")
	}

	// Open a memoryset by name or ID
	ms, err := client.OpenMemoryset(ctx, "vacuum_cleaner_review_sentiment_classifier_2193")
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
	results, err := ms.Search(ctx, "I like the vacuum cleaner", orca.WithSearchCount(5))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Search results:")
	for _, r := range results {
		fmt.Printf("  %s (score: %.3f)\n", r.Value, r.LookupScore)
	}

	// Classification prediction
	model, err := client.OpenClassificationModel(ctx, "vacuum_cleaner_review_sentiment_classifier_2193")
	if err != nil {
		log.Fatal(err)
	}
	pred, err := model.Predict(ctx, "this is the worst product I have ever used")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Label: %d, Confidence: %.3f\n", *pred.Label, pred.Confidence)

	// Batched predictions — individual Predict calls are automatically
	// grouped into batch requests, flushing when the batch reaches 10
	// items or after 1000ms, whichever comes first.
	batcher := model.NewBatcher(
		[]orca.BatcherOption{
			orca.WithBatchSize(10),
			orca.WithBatchDelay(1000 * time.Millisecond),
		},
	)
	defer batcher.Close()

	reviews := []string{
		"Absolutely love this vacuum cleaner!",
		"Terrible suction, broke after a week.",
		"Decent for the price, nothing special.",
		"Best purchase I've made this year.",
		"Would not recommend to anyone.",
	}

	var wg sync.WaitGroup
	for _, review := range reviews {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			p, err := batcher.Predict(ctx, text)
			if err != nil {
				log.Printf("prediction error: %v", err)
				return
			}
			fmt.Printf("Review: %q → Label: %d, Confidence: %.3f\n", text, *p.Label, p.Confidence)
		}(review)
	}
	wg.Wait()
}
