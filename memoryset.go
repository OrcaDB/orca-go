package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// Memoryset is a handle to a remote memoryset in Orca.
type Memoryset struct {
	client   *Client
	Metadata MemorysetMetadata
}

// OpenMemoryset opens an existing memoryset by name or ID.
func (c *Client) OpenMemoryset(ctx context.Context, nameOrID string) (*Memoryset, error) {
	data, err := c.get(ctx, "/memoryset/"+url.PathEscape(nameOrID), nil)
	if err != nil {
		return nil, err
	}
	var meta MemorysetMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing memoryset metadata: %w", err)
	}
	return &Memoryset{client: c, Metadata: meta}, nil
}

// ListMemorysets lists all memorysets visible to the current API key.
func (c *Client) ListMemorysets(ctx context.Context) ([]MemorysetMetadata, error) {
	data, err := c.get(ctx, "/memoryset", nil)
	if err != nil {
		return nil, err
	}
	var metas []MemorysetMetadata
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("parsing memoryset list: %w", err)
	}
	return metas, nil
}

// Count returns the number of memories from the last-fetched metadata.
// Call Refresh to get the current count from the server.
func (m *Memoryset) Count() int {
	return m.Metadata.Length
}

// Refresh re-fetches the memoryset metadata from the server.
func (m *Memoryset) Refresh(ctx context.Context) error {
	data, err := m.client.get(ctx, "/memoryset/"+url.PathEscape(m.Metadata.ID), nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &m.Metadata); err != nil {
		return fmt.Errorf("parsing memoryset metadata: %w", err)
	}
	return nil
}

// --- Query ---

type queryParams struct {
	Offset           int      `json:"offset"`
	Limit            int      `json:"limit"`
	Filters          []Filter `json:"filters"`
	ConsistencyLevel string   `json:"consistency_level"`
}

// QueryOption configures a Query call.
type QueryOption func(*queryParams)

// WithOffset sets the pagination offset (default 0).
func WithOffset(n int) QueryOption {
	return func(p *queryParams) { p.Offset = n }
}

// WithLimit sets the maximum number of results (default 100).
func WithLimit(n int) QueryOption {
	return func(p *queryParams) { p.Limit = n }
}

// WithFilters sets filter conditions for the query.
func WithFilters(filters ...Filter) QueryOption {
	return func(p *queryParams) { p.Filters = filters }
}

// WithConsistencyLevel sets the consistency level.
// Valid values: "Strong", "Session", "Bounded", "Eventual".
func WithConsistencyLevel(level string) QueryOption {
	return func(p *queryParams) { p.ConsistencyLevel = level }
}

// Query lists memories in the memoryset with optional filtering and pagination.
func (m *Memoryset) Query(ctx context.Context, opts ...QueryOption) ([]Memory, error) {
	p := &queryParams{
		Offset:           0,
		Limit:            100,
		Filters:          []Filter{},
		ConsistencyLevel: "Bounded",
	}
	for _, opt := range opts {
		opt(p)
	}

	data, err := m.client.post(ctx, "/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories", p)
	if err != nil {
		return nil, err
	}
	var memories []Memory
	if err := json.Unmarshal(data, &memories); err != nil {
		return nil, fmt.Errorf("parsing memories: %w", err)
	}
	return memories, nil
}

// --- Search ---

type searchParams struct {
	Query               []string `json:"query"`
	Count               int      `json:"count"`
	Prompt              *string  `json:"prompt,omitempty"`
	PartitionID         *string  `json:"partition_id,omitempty"`
	PartitionFilterMode string   `json:"partition_filter_mode"`
	ConsistencyLevel    string   `json:"consistency_level"`
}

// SearchOption configures a Search or SearchBatch call.
type SearchOption func(*searchParams)

// WithSearchCount sets the number of results per query (default 1).
func WithSearchCount(n int) SearchOption {
	return func(p *searchParams) { p.Count = n }
}

// WithSearchPrompt sets an optional prompt to contextualize the search.
func WithSearchPrompt(prompt string) SearchOption {
	return func(p *searchParams) { p.Prompt = &prompt }
}

// WithSearchPartitionID restricts search to a specific partition.
func WithSearchPartitionID(id string) SearchOption {
	return func(p *searchParams) { p.PartitionID = &id }
}

// WithSearchConsistencyLevel sets the consistency level for the search.
func WithSearchConsistencyLevel(level string) SearchOption {
	return func(p *searchParams) { p.ConsistencyLevel = level }
}

// Search performs a semantic similarity search with a single query string.
func (m *Memoryset) Search(ctx context.Context, query string, opts ...SearchOption) ([]MemoryLookup, error) {
	results, err := m.SearchBatch(ctx, []string{query}, opts...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// SearchBatch performs a semantic similarity search with multiple query strings.
// Returns one slice of results per query.
func (m *Memoryset) SearchBatch(ctx context.Context, queries []string, opts ...SearchOption) ([][]MemoryLookup, error) {
	p := &searchParams{
		Query:               queries,
		Count:               1,
		PartitionFilterMode: "include_global",
		ConsistencyLevel:    "Bounded",
	}
	for _, opt := range opts {
		opt(p)
	}

	data, err := m.client.post(ctx, "/gpu/memoryset/"+url.PathEscape(m.Metadata.ID)+"/lookup", p)
	if err != nil {
		return nil, err
	}
	var results [][]MemoryLookup
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	return results, nil
}

// Get retrieves one or more memories by their IDs.
func (m *Memoryset) Get(ctx context.Context, memoryIDs ...string) ([]Memory, error) {
	body := struct {
		MemoryIDs        []string `json:"memory_ids"`
		ConsistencyLevel string   `json:"consistency_level"`
	}{
		MemoryIDs:        memoryIDs,
		ConsistencyLevel: "Bounded",
	}
	data, err := m.client.post(ctx, "/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories/get", body)
	if err != nil {
		return nil, err
	}
	var memories []Memory
	if err := json.Unmarshal(data, &memories); err != nil {
		return nil, fmt.Errorf("parsing memories: %w", err)
	}
	return memories, nil
}

// Insert adds memories to the memoryset and returns their assigned IDs.
func (m *Memoryset) Insert(ctx context.Context, items []MemoryInsert) ([]string, error) {
	data, err := m.client.post(ctx, "/gpu/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memory", items)
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("parsing insert response: %w", err)
	}
	return ids, nil
}

// Update modifies specific memories identified by their MemoryID fields.
// Returns the number of updated memories.
func (m *Memoryset) Update(ctx context.Context, updates []MemoryUpdate) (int, error) {
	body := struct {
		Updates []MemoryUpdate `json:"updates"`
	}{Updates: updates}

	data, err := m.client.patch(ctx, "/gpu/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		UpdatedCount int `json:"updated_count"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing update response: %w", err)
	}
	return resp.UpdatedCount, nil
}

// UpdateByFilter modifies all memories matching the given filters.
// Returns the number of updated memories.
func (m *Memoryset) UpdateByFilter(ctx context.Context, filters []Filter, patch MemoryPatch) (int, error) {
	body := struct {
		Filters []Filter    `json:"filters"`
		Patch   MemoryPatch `json:"patch"`
	}{Filters: filters, Patch: patch}

	data, err := m.client.patch(ctx, "/gpu/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		UpdatedCount int `json:"updated_count"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing update response: %w", err)
	}
	return resp.UpdatedCount, nil
}

// Delete removes memories by their IDs.
// Returns the number of deleted memories.
func (m *Memoryset) Delete(ctx context.Context, memoryIDs ...string) (int, error) {
	body := struct {
		MemoryIDs []string `json:"memory_ids"`
	}{MemoryIDs: memoryIDs}

	data, err := m.client.post(ctx, "/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories/delete", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		DeletedCount int `json:"deleted_count"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing delete response: %w", err)
	}
	return resp.DeletedCount, nil
}

// DeleteByFilter removes all memories matching the given filters.
// Returns the number of deleted memories.
func (m *Memoryset) DeleteByFilter(ctx context.Context, filters []Filter) (int, error) {
	body := struct {
		Filters []Filter `json:"filters"`
	}{Filters: filters}

	data, err := m.client.post(ctx, "/memoryset/"+url.PathEscape(m.Metadata.ID)+"/memories/delete", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		DeletedCount int `json:"deleted_count"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, fmt.Errorf("parsing delete response: %w", err)
	}
	return resp.DeletedCount, nil
}

// Truncate removes all memories from the memoryset.
// Returns the number of deleted memories.
func (m *Memoryset) Truncate(ctx context.Context) (int, error) {
	return m.DeleteByFilter(ctx, []Filter{})
}
