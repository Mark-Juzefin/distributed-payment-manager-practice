package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensearch-project/opensearch-go"
)

type indexer struct {
	client *opensearch.Client
	index  string
}

func newIndexer(urls []string, index string) (*indexer, error) {
	if len(urls) == 0 {
		return nil, errors.New("no OpenSearch addresses configured")
	}

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: urls,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opensearch client: %w", err)
	}

	idx := &indexer{client: client, index: index}
	if err := idx.ensureIndex(context.Background()); err != nil {
		return nil, err
	}
	return idx, nil
}

// ensureIndex creates the index with mapping if it does not exist yet.
func (idx *indexer) ensureIndex(ctx context.Context) error {
	res, err := idx.client.Indices.Exists(
		[]string{idx.index},
		idx.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("indices.exists: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		return nil
	}

	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"id":              map[string]any{"type": "keyword"},
				"aggregate_type":  map[string]any{"type": "keyword"},
				"aggregate_id":    map[string]any{"type": "keyword"},
				"event_type":      map[string]any{"type": "keyword"},
				"idempotency_key": map[string]any{"type": "keyword"},
				"payload":         map[string]any{"type": "object", "enabled": true},
				"created_at":      map[string]any{"type": "date"},
			},
		},
		"settings": map[string]any{
			"number_of_replicas": 0,
		},
	}
	buf, _ := json.Marshal(mapping)

	cr, err := idx.client.Indices.Create(
		idx.index,
		idx.client.Indices.Create.WithBody(bytes.NewReader(buf)),
		idx.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("indices.create: %w", err)
	}
	defer cr.Body.Close()
	if cr.IsError() {
		return fmt.Errorf("indices.create error: %s", cr.String())
	}
	return nil
}

// indexEvent indexes a single event document. Uses event ID as the document
// _id, making re-indexing idempotent (same data overwrites itself).
func (idx *indexer) indexEvent(ctx context.Context, evt event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	res, err := idx.client.Index(
		idx.index,
		bytes.NewReader(payload),
		idx.client.Index.WithDocumentID(evt.ID),
		idx.client.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index error: %s", res.String())
	}
	return nil
}
