package opensearch

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/opensearch-project/opensearch-go"
)

var _ dispute.EventSink = (*EventSink)(nil)

type EventSink struct {
	client        *opensearch.Client
	indexDisputes string
}

// TODO: refactor - total rewrite and replane
func NewOpenSearchEventSink(ctx context.Context, urls []string, indexDisputes, indexOrders string) (*EventSink, error) {
	if len(urls) == 0 {
		return nil, errors.New("no OpenSearch addresses configured")
	}

	cfg := opensearch.Config{
		Addresses: urls,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
		},
	}
	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("opensearch client: %w", err)
	}

	sink := &EventSink{client: client, indexDisputes: indexDisputes}

	// Ensure index exists with minimal mapping.
	if err := sink.ensureIndex(ctx); err != nil {
		return nil, err
	}
	return sink, nil
}

func (s *EventSink) ensureIndex(ctx context.Context) error {
	// HEAD /{index}
	res, err := s.client.Indices.Exists([]string{s.indexDisputes}, s.client.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("indices.exists: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		return nil // already exists
	}
	// Create index with a simple mapping.
	body := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"event_id":          map[string]any{"type": "keyword"},
				"entity_id":         map[string]any{"type": "keyword"}, // dispute_id
				"kind":              map[string]any{"type": "keyword"},
				"provider_event_id": map[string]any{"type": "keyword"},
				"created_at":        map[string]any{"type": "date"},
				"data":              map[string]any{"type": "object", "enabled": true},
			},
		},
		"settings": map[string]any{
			"number_of_replicas": 0, // dev-friendly; change in prod
		},
	}
	buf, _ := json.Marshal(body)
	cr, err := s.client.Indices.Create(
		s.indexDisputes,
		s.client.Indices.Create.WithBody(bytes.NewReader(buf)),
		s.client.Indices.Create.WithContext(ctx),
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

// internal doc stored in OpenSearch
type osDisputeEventDoc struct {
	EventID         string                   `json:"event_id"`
	EntityID        string                   `json:"entity_id"` // stores DisputeID
	Kind            dispute.DisputeEventKind `json:"kind"`
	ProviderEventID string                   `json:"provider_event_id,omitempty"`
	Data            json.RawMessage          `json:"data,omitempty"`
	CreatedAt       time.Time                `json:"created_at"`
}

func (s *EventSink) CreateDisputeEvent(ctx context.Context, ev dispute.NewDisputeEvent) (*dispute.DisputeEvent, error) {
	// Generate event ID here (since NewDisputeEvent has none).
	eventID := uuid.NewString()
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	doc := osDisputeEventDoc{
		EventID:         eventID,
		EntityID:        ev.DisputeID,
		Kind:            ev.Kind,
		ProviderEventID: ev.ProviderEventID,
		Data:            ev.Data,
		CreatedAt:       ev.CreatedAt.UTC(),
	}
	payload, _ := json.Marshal(doc)
	res, err := s.client.Index(
		s.indexDisputes,
		bytes.NewReader(payload),
		s.client.Index.WithDocumentID(eventID),
		s.client.Index.WithContext(ctx),
		// In dev you can force refresh so reads see writes immediately. Remove for prod perf.
		s.client.Index.WithRefresh("true"),
	)
	if err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("index error: %s", res.String())
	}
	return &dispute.DisputeEvent{
		EventID:         eventID,
		NewDisputeEvent: ev,
	}, nil
}

func (s *EventSink) GetDisputeEvents(ctx context.Context, query dispute.DisputeEventQuery) (dispute.DisputeEventPage, error) {
	// Build a simple bool/filter query.
	filters := make([]map[string]any, 0, 2)
	if len(query.DisputeIDs) > 0 {
		// terms on entity_id
		vals := make([]string, 0, len(query.DisputeIDs))
		for _, id := range query.DisputeIDs {
			if id != "" {
				vals = append(vals, id)
			}
		}
		if len(vals) > 0 {
			filters = append(filters, map[string]any{
				"terms": map[string]any{"entity_id": vals},
			})
		}
	}
	if len(query.Kinds) > 0 {
		vals := make([]string, 0, len(query.Kinds))
		for _, k := range query.Kinds {
			if k != "" {
				vals = append(vals, string(k))
			}
		}
		if len(vals) > 0 {
			filters = append(filters, map[string]any{
				"terms": map[string]any{"kind": vals},
			})
		}
	}

	body := map[string]any{
		"size": 500, // tune as needed
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		},
		"sort": []map[string]any{
			{"created_at": map[string]any{"order": "asc"}},
		},
	}
	raw, _ := json.Marshal(body)

	res, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.indexDisputes),
		s.client.Search.WithBody(bytes.NewReader(raw)),
	)
	if err != nil {
		return dispute.DisputeEventPage{}, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return dispute.DisputeEventPage{}, fmt.Errorf("search error: %s", res.String())
	}

	var sr struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&sr); err != nil {
		return dispute.DisputeEventPage{}, fmt.Errorf("decode search: %w", err)
	}

	out := make([]dispute.DisputeEvent, 0, len(sr.Hits.Hits))
	for _, h := range sr.Hits.Hits {
		var doc osDisputeEventDoc
		if err := json.Unmarshal(h.Source, &doc); err != nil {
			return dispute.DisputeEventPage{}, fmt.Errorf("decode hit: %w", err)
		}
		evtID := doc.EventID
		if evtID == "" {
			// fallback to _id if source didn't contain event_id for some reason
			evtID = h.ID
		}
		out = append(out, dispute.DisputeEvent{
			EventID: evtID,
			NewDisputeEvent: dispute.NewDisputeEvent{
				DisputeID:       doc.EntityID,
				Kind:            doc.Kind,
				ProviderEventID: doc.ProviderEventID,
				Data:            doc.Data,
				CreatedAt:       doc.CreatedAt,
			},
		})
	}
	return dispute.DisputeEventPage{
		Items:      out,
		NextCursor: "",    //TODO
		HasMore:    false, //TODO
	}, nil
}
