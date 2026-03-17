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
}
