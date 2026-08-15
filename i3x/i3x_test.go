package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupMockServer(t *testing.T) (*httptest.Server, *Client) {
	mux := http.NewServeMux()

	// GET /v1/info
	mux.HandleFunc("/v1/info", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sName := "Mock i3X Server"
		sVer := "1.0-test"
		resp := SuccessResponse[ServerInfo]{
			Success: true,
			Result: ServerInfo{
				SpecVersion:   "1.0",
				ServerVersion: &sVer,
				ServerName:    &sName,
				Capabilities: ServerCapabilities{
					Query: struct {
						History bool `json:"history"`
					}{History: true},
					Update: struct {
						Current bool `json:"current"`
						History bool `json:"history"`
					}{Current: true, History: true},
					Subscribe: struct {
						Stream bool `json:"stream"`
					}{Stream: true},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /v1/namespaces
	mux.HandleFunc("/v1/namespaces", func(w http.ResponseWriter, r *http.Request) {
		resp := SuccessResponse[[]Namespace]{
			Success: true,
			Result: []Namespace{
				{URI: "https://cesmii.org/i3x", DisplayName: "I3X"},
				{URI: "https://isa.org/isa95", DisplayName: "ISA95"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /v1/objecttypes
	mux.HandleFunc("/v1/objecttypes", func(w http.ResponseWriter, r *http.Request) {
		resp := SuccessResponse[[]ObjectTypeResponse]{
			Success: true,
			Result: []ObjectTypeResponse{
				{
					ElementID:    "sensor-type",
					DisplayName:  "SensorType",
					NamespaceURI: "https://thinkiq.com/equipment",
					SourceTypeID: "1001",
					Schema:       map[string]interface{}{"type": "object"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/objecttypes/query
	mux.HandleFunc("/v1/objecttypes/query", func(w http.ResponseWriter, r *http.Request) {
		var req GetObjectTypesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		resp := BulkResponse[ObjectTypeResponse]{
			Success: true,
			Results: []BulkResultItem[ObjectTypeResponse]{
				{
					Success:   true,
					ElementID: &id,
					Result: &ObjectTypeResponse{
						ElementID:    id,
						DisplayName:  "QueriedType",
						NamespaceURI: "https://thinkiq.com/equipment",
						SourceTypeID: "1001",
						Schema:       map[string]interface{}{"type": "object"},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /v1/relationshiptypes
	mux.HandleFunc("/v1/relationshiptypes", func(w http.ResponseWriter, r *http.Request) {
		resp := SuccessResponse[[]RelationshipType]{
			Success: true,
			Result: []RelationshipType{
				{
					ElementID:      "isa95-has-part",
					DisplayName:    "HasPart",
					NamespaceURI:   "https://isa.org/isa95",
					RelationshipID: "hasPart",
					ReverseOf:      "isPartOf",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/relationshiptypes/query
	mux.HandleFunc("/v1/relationshiptypes/query", func(w http.ResponseWriter, r *http.Request) {
		var req GetRelationshipTypesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		resp := BulkResponse[RelationshipType]{
			Success: true,
			Results: []BulkResultItem[RelationshipType]{
				{
					Success:   true,
					ElementID: &id,
					Result: &RelationshipType{
						ElementID:      id,
						DisplayName:    "HasPart",
						NamespaceURI:   "https://isa.org/isa95",
						RelationshipID: "hasPart",
						ReverseOf:      "isPartOf",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// GET /v1/objects
	mux.HandleFunc("/v1/objects", func(w http.ResponseWriter, r *http.Request) {
		resp := SuccessResponse[[]ObjectInstanceResponse]{
			Success: true,
			Result: []ObjectInstanceResponse{
				{
					ElementID:     "pump-station",
					DisplayName:   "Pump Station",
					TypeElementID: "work-center-type",
					IsComposition: false,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/objects/list
	mux.HandleFunc("/v1/objects/list", func(w http.ResponseWriter, r *http.Request) {
		var req GetObjectsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		resp := BulkResponse[ObjectInstanceResponse]{
			Success: true,
			Results: []BulkResultItem[ObjectInstanceResponse]{
				{
					Success:   true,
					ElementID: &id,
					Result: &ObjectInstanceResponse{
						ElementID:     id,
						DisplayName:   "Pump Station",
						TypeElementID: "work-center-type",
						IsComposition: false,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/objects/related
	mux.HandleFunc("/v1/objects/related", func(w http.ResponseWriter, r *http.Request) {
		var req GetRelatedObjectsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		rel := []RelatedObjectResult{
			{
				SourceRelationship: "HasChildren",
				Object: ObjectInstanceResponse{
					ElementID:     "pump-101",
					DisplayName:   "Pump 101",
					TypeElementID: "work-unit-type",
					IsComposition: true,
				},
			},
		}
		resp := BulkResponse[[]RelatedObjectResult]{
			Success: true,
			Results: []BulkResultItem[[]RelatedObjectResult]{
				{
					Success:   true,
					ElementID: &id,
					Result:    &rel,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST & PUT /v1/objects/value
	mux.HandleFunc("/v1/objects/value", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req GetObjectValueRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			id := req.ElementIDs[0]
			resp := BulkResponse[CurrentValueResult]{
				Success: true,
				Results: []BulkResultItem[CurrentValueResult]{
					{
						Success:   true,
						ElementID: &id,
						Result: &CurrentValueResult{
							IsComposition: false,
							Value:         42.5,
							Quality:       "Good",
							Timestamp:     "2026-08-15T08:00:00Z",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPut {
			var req UpdateValueRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			id := req.Updates[0].ElementID
			resp := BulkResponse[interface{}]{
				Success: true,
				Results: []BulkResultItem[interface{}]{
					{
						Success:   true,
						ElementID: &id,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	})

	// POST & PUT /v1/objects/history
	mux.HandleFunc("/v1/objects/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var req GetObjectHistoryRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			id := req.ElementIDs[0]
			resp := BulkResponse[HistoricalValueResult]{
				Success: true,
				Results: []BulkResultItem[HistoricalValueResult]{
					{
						Success:   true,
						ElementID: &id,
						Result: &HistoricalValueResult{
							IsComposition: false,
							Values: []VQT{
								{Value: 40.0, Quality: "Good", Timestamp: "2026-08-15T07:00:00Z"},
								{Value: 42.5, Quality: "Good", Timestamp: "2026-08-15T08:00:00Z"},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == http.MethodPut {
			var req UpdateHistoryRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			id := req.Updates[0].ElementID
			resp := BulkResponse[interface{}]{
				Success: true,
				Results: []BulkResultItem[interface{}]{
					{
						Success:   true,
						ElementID: &id,
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
	})

	// POST /v1/subscriptions
	mux.HandleFunc("/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		var req CreateSubscriptionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		name := "TestSub"
		resp := SuccessResponse[CreateSubscriptionResponse]{
			Success: true,
			Result: CreateSubscriptionResponse{
				ClientID:       req.ClientID,
				SubscriptionID: "sub-123-abc",
				DisplayName:    &name,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/register
	mux.HandleFunc("/v1/subscriptions/register", func(w http.ResponseWriter, r *http.Request) {
		var req RegisterMonitoredItemsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		resp := BulkResponse[interface{}]{
			Success: true,
			Results: []BulkResultItem[interface{}]{
				{
					Success:   true,
					ElementID: &id,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/unregister
	mux.HandleFunc("/v1/subscriptions/unregister", func(w http.ResponseWriter, r *http.Request) {
		var req UnregisterMonitoredItemsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := req.ElementIDs[0]
		resp := BulkResponse[interface{}]{
			Success: true,
			Results: []BulkResultItem[interface{}]{
				{
					Success:   true,
					ElementID: &id,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/sync
	mux.HandleFunc("/v1/subscriptions/sync", func(w http.ResponseWriter, r *http.Request) {
		resp := SuccessResponse[[]SyncBatch]{
			Success: true,
			Result: []SyncBatch{
				{
					SequenceNumber: 1,
					Updates: []SyncUpdateEntry{
						{
							ElementID: "pump-station",
							Value:     99.9,
							Quality:   "Good",
							Timestamp: "2026-08-15T08:00:00Z",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/list
	mux.HandleFunc("/v1/subscriptions/list", func(w http.ResponseWriter, r *http.Request) {
		var req ListSubscriptionsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		subID := req.SubscriptionIDs[0]
		depth := 1
		resp := BulkResponse[SubscriptionDetail]{
			Success: true,
			Results: []BulkResultItem[SubscriptionDetail]{
				{
					Success:        true,
					SubscriptionID: &subID,
					Result: &SubscriptionDetail{
						SubscriptionID: subID,
						MonitoredObjects: []MonitoredObject{
							{ElementID: "pump-station", MaxDepth: &depth},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/delete
	mux.HandleFunc("/v1/subscriptions/delete", func(w http.ResponseWriter, r *http.Request) {
		var req DeleteSubscriptionsRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		subID := req.SubscriptionIDs[0]
		resp := BulkResponse[interface{}]{
			Success: true,
			Results: []BulkResultItem[interface{}]{
				{
					Success:        true,
					SubscriptionID: &subID,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// POST /v1/subscriptions/stream (SSE)
	mux.HandleFunc("/v1/subscriptions/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		eventData := `{"sequenceNumber": 1, "updates": [{"elementId": "pump-station", "value": 100, "quality": "Good", "timestamp": "2026-08-15T08:00:00Z"}]}`
		fmt.Fprintf(w, "event: update\ndata: %s\n\n", eventData)
		flusher.Flush()
	})

	server := httptest.NewServer(mux)

	cfg := ClientConfig{
		BaseURL:  server.URL,
		ClientID: "test-client",
		Timeout:  5 * time.Second,
	}
	client := NewClient(cfg)

	return server, client
}

func TestAllEndpoints(t *testing.T) {
	server, client := setupMockServer(t)
	defer server.Close()

	ctx := context.Background()

	// 1. Info
	info, err := client.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo failed: %v", err)
	}
	if info.SpecVersion != "1.0" {
		t.Errorf("expected specVersion 1.0, got %s", info.SpecVersion)
	}

	// 2. Namespaces
	ns, err := client.GetNamespaces(ctx)
	if err != nil {
		t.Fatalf("GetNamespaces failed: %v", err)
	}
	if len(ns) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(ns))
	}

	// 3. Object Types
	ot, err := client.GetObjectTypes(ctx, "")
	if err != nil {
		t.Fatalf("GetObjectTypes failed: %v", err)
	}
	if len(ot) != 1 || ot[0].ElementID != "sensor-type" {
		t.Errorf("unexpected ObjectTypes: %+v", ot)
	}

	// 4. Query Object Types
	otq, err := client.QueryObjectTypes(ctx, []string{"sensor-type"})
	if err != nil {
		t.Fatalf("QueryObjectTypes failed: %v", err)
	}
	if len(otq.Results) != 1 || !otq.Results[0].Success {
		t.Errorf("unexpected QueryObjectTypes: %+v", otq)
	}

	// 5. Relationship Types
	rt, err := client.GetRelationshipTypes(ctx, "")
	if err != nil {
		t.Fatalf("GetRelationshipTypes failed: %v", err)
	}
	if len(rt) != 1 || rt[0].ElementID != "isa95-has-part" {
		t.Errorf("unexpected RelationshipTypes: %+v", rt)
	}

	// 6. Query Relationship Types
	rtq, err := client.QueryRelationshipTypes(ctx, []string{"isa95-has-part"})
	if err != nil {
		t.Fatalf("QueryRelationshipTypes failed: %v", err)
	}
	if len(rtq.Results) != 1 || !rtq.Results[0].Success {
		t.Errorf("unexpected QueryRelationshipTypes: %+v", rtq)
	}

	// 7. Objects
	objs, err := client.GetObjects(ctx, "", false, nil)
	if err != nil {
		t.Fatalf("GetObjects failed: %v", err)
	}
	if len(objs) != 1 || objs[0].ElementID != "pump-station" {
		t.Errorf("unexpected GetObjects: %+v", objs)
	}

	// 8. List Objects
	objList, err := client.ListObjects(ctx, []string{"pump-station"}, false)
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(objList.Results) != 1 || !objList.Results[0].Success {
		t.Errorf("unexpected ListObjects: %+v", objList)
	}

	// 9. Related Objects
	rel, err := client.QueryRelatedObjects(ctx, []string{"pump-station"}, "", false)
	if err != nil {
		t.Fatalf("QueryRelatedObjects failed: %v", err)
	}
	if len(rel.Results) != 1 || len(*rel.Results[0].Result) != 1 {
		t.Errorf("unexpected QueryRelatedObjects: %+v", rel)
	}

	// 10. Query Last Known Values
	val, err := client.QueryLastKnownValues(ctx, []string{"pump-station"}, 1)
	if err != nil {
		t.Fatalf("QueryLastKnownValues failed: %v", err)
	}
	if len(val.Results) != 1 || val.Results[0].Result.Value != 42.5 {
		t.Errorf("unexpected QueryLastKnownValues: %+v", val)
	}

	// 11. Update Values
	valUp, err := client.UpdateObjectValues(ctx, []ValueUpdateItem{
		{ElementID: "pump-station", Value: VQTInput{Value: 50.0}},
	})
	if err != nil {
		t.Fatalf("UpdateObjectValues failed: %v", err)
	}
	if len(valUp.Results) != 1 || !valUp.Results[0].Success {
		t.Errorf("unexpected UpdateObjectValues: %+v", valUp)
	}

	// 12. Query History
	hist, err := client.QueryHistoricalValues(ctx, []string{"pump-station"}, "2026-08-15T07:00:00Z", "2026-08-15T08:00:00Z", 1)
	if err != nil {
		t.Fatalf("QueryHistoricalValues failed: %v", err)
	}
	if len(hist.Results) != 1 || len(hist.Results[0].Result.Values) != 2 {
		t.Errorf("unexpected QueryHistoricalValues: %+v", hist)
	}

	// 13. Update History
	histUp, err := client.UpdateObjectHistory(ctx, []HistoryUpdateItem{
		{ElementID: "pump-station", Value: VQT{Value: 55.0, Quality: "Good", Timestamp: "2026-08-15T09:00:00Z"}},
	})
	if err != nil {
		t.Fatalf("UpdateObjectHistory failed: %v", err)
	}
	if len(histUp.Results) != 1 || !histUp.Results[0].Success {
		t.Errorf("unexpected UpdateObjectHistory: %+v", histUp)
	}

	// 14. Create Subscription
	sub, err := client.CreateSubscription(ctx, "test-client", "TestSub")
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	if sub.SubscriptionID != "sub-123-abc" {
		t.Errorf("unexpected subscription ID: %s", sub.SubscriptionID)
	}

	// 15. Register Monitored Items
	reg, err := client.RegisterMonitoredItems(ctx, "test-client", sub.SubscriptionID, []string{"pump-station"}, nil)
	if err != nil {
		t.Fatalf("RegisterMonitoredItems failed: %v", err)
	}
	if len(reg.Results) != 1 || !reg.Results[0].Success {
		t.Errorf("unexpected RegisterMonitoredItems: %+v", reg)
	}

	// 16. List Subscriptions
	subList, err := client.ListSubscriptions(ctx, "test-client", []string{sub.SubscriptionID})
	if err != nil {
		t.Fatalf("ListSubscriptions failed: %v", err)
	}
	if len(subList.Results) != 1 || !subList.Results[0].Success {
		t.Errorf("unexpected ListSubscriptions: %+v", subList)
	}

	// 17. Sync Subscription
	sync, err := client.SyncSubscription(ctx, "test-client", sub.SubscriptionID, nil)
	if err != nil {
		t.Fatalf("SyncSubscription failed: %v", err)
	}
	if len(sync) != 1 || sync[0].SequenceNumber != 1 {
		t.Errorf("unexpected SyncSubscription: %+v", sync)
	}

	// 18. Stream Subscription (SSE)
	var streamedEvents []SSEEvent
	streamCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	_ = client.StreamSubscription(streamCtx, "test-client", sub.SubscriptionID, func(ev SSEEvent) error {
		streamedEvents = append(streamedEvents, ev)
		cancel() // Stop after 1 event
		return nil
	})
	if len(streamedEvents) == 0 {
		t.Errorf("expected at least 1 streamed SSE event")
	}

	// 19. Unregister Monitored Items
	unreg, err := client.UnregisterMonitoredItems(ctx, "test-client", sub.SubscriptionID, []string{"pump-station"})
	if err != nil {
		t.Fatalf("UnregisterMonitoredItems failed: %v", err)
	}
	if len(unreg.Results) != 1 || !unreg.Results[0].Success {
		t.Errorf("unexpected UnregisterMonitoredItems: %+v", unreg)
	}

	// 20. Delete Subscription
	del, err := client.DeleteSubscriptions(ctx, "test-client", []string{sub.SubscriptionID})
	if err != nil {
		t.Fatalf("DeleteSubscriptions failed: %v", err)
	}
	if len(del.Results) != 1 || !del.Results[0].Success {
		t.Errorf("unexpected DeleteSubscriptions: %+v", del)
	}
}

func TestCommandExecution(t *testing.T) {
	server, client := setupMockServer(t)
	defer server.Close()

	var buf bytes.Buffer
	formatter := &Formatter{
		Format:  FormatTable,
		NoColor: true,
		Out:     &buf,
	}
	handler := NewCommandHandler(client, formatter)
	ctx := context.Background()

	commandsToTest := [][]string{
		{"info"},
		{"namespaces"},
		{"types"},
		{"types", "query", "sensor-type"},
		{"rel-types"},
		{"rel-types", "query", "isa95-has-part"},
		{"objects"},
		{"objects", "query", "pump-station"},
		{"related", "pump-station"},
		{"read", "pump-station"},
		{"write", "pump-station", "45.0"},
		{"history", "pump-station", "--start", "-1h"},
		{"write-history", "pump-station", "48.0"},
		{"sub", "create", "--name", "Test"},
		{"sub", "list", "sub-123-abc"},
		{"sub", "register", "sub-123-abc", "pump-station"},
		{"sub", "sync", "sub-123-abc"},
		{"sub", "unregister", "sub-123-abc", "pump-station"},
		{"sub", "delete", "sub-123-abc"},
	}

	for _, cmd := range commandsToTest {
		buf.Reset()
		err := handler.Execute(ctx, cmd)
		if err != nil {
			t.Errorf("command execution failed for %v: %v", cmd, err)
		}
		if buf.Len() == 0 {
			t.Errorf("command %v produced empty output", cmd)
		}
	}
}

func TestCommandLineParser(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    `write pump-1 "hello world" --quality Good`,
			expected: []string{"write", "pump-1", "hello world", "--quality", "Good"},
		},
		{
			input:    `sub create --name 'My Sensor Sub'`,
			expected: []string{"sub", "create", "--name", "My Sensor Sub"},
		},
		{
			input:    `read id1 id2`,
			expected: []string{"read", "id1", "id2"},
		},
	}

	for _, tc := range tests {
		actual := parseCommandLine(tc.input)
		if len(actual) != len(tc.expected) {
			t.Fatalf("for %q, expected %d parts, got %d: %v", tc.input, len(tc.expected), len(actual), actual)
		}
		for i := range actual {
			if actual[i] != tc.expected[i] {
				t.Errorf("for %q, part %d: expected %q, got %q", tc.input, i, tc.expected[i], actual[i])
			}
		}
	}
}

func TestTimeParser(t *testing.T) {
	now := time.Now()
	res := parseOrFormatTimestamp("-1h")
	parsed, err := time.Parse(time.RFC3339Nano, res)
	if err != nil {
		t.Fatalf("failed to parse result of -1h: %v", err)
	}
	diff := now.Sub(parsed)
	if diff < 55*time.Minute || diff > 65*time.Minute {
		t.Errorf("unexpected diff: %v", diff)
	}
}

func TestCmdObjectsFilters(t *testing.T) {
	server, client := setupMockServer(t)
	defer server.Close()

	var buf bytes.Buffer
	formatter := NewFormatter(FormatTable, false)
	formatter.Out = &buf
	handler := NewCommandHandler(client, formatter)
	ctx := context.Background()

	// 1. Help flag
	if err := handler.Execute(ctx, []string{"objects", "-h"}); err != nil {
		t.Errorf("expected nil error for help, got %v", err)
	}

	// 2. Filter by pattern match
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--filter", "*pump*"}); err != nil {
		t.Fatalf("filter *pump* failed: %v", err)
	}
	if !strings.Contains(buf.String(), "pump-station") {
		t.Errorf("expected pump-station in output: %s", buf.String())
	}

	// 3. Filter by non-matching pattern
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--filter", "nonexistent"}); err != nil {
		t.Fatalf("filter nonexistent failed: %v", err)
	}
	if strings.Contains(buf.String(), "pump-station") {
		t.Errorf("expected no pump-station in output: %s", buf.String())
	}

	// 4. Filter by name
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--name", "Pump"}); err != nil {
		t.Fatalf("filter name failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Pump Station") {
		t.Errorf("expected Pump Station in output: %s", buf.String())
	}

	// 5. Filter by type
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--type", "work-center-type"}); err != nil {
		t.Fatalf("filter type failed: %v", err)
	}
	if !strings.Contains(buf.String(), "pump-station") {
		t.Errorf("expected pump-station in output: %s", buf.String())
	}

	// 6. Test --root and --includeMetadata flags
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--root", "--includeMetadata"}); err != nil {
		t.Fatalf("objects --root --includeMetadata failed: %v", err)
	}
	if !strings.Contains(buf.String(), "pump-station") {
		t.Errorf("expected pump-station in output: %s", buf.String())
	}

	// 7. Test --root=true and --include-metadata
	buf.Reset()
	if err := handler.Execute(ctx, []string{"objects", "--root=true", "--include-metadata"}); err != nil {
		t.Fatalf("objects --root=true --include-metadata failed: %v", err)
	}
	if !strings.Contains(buf.String(), "pump-station") {
		t.Errorf("expected pump-station in output: %s", buf.String())
	}
}

func TestAllCommandHelpFlags(t *testing.T) {
	server, client := setupMockServer(t)
	defer server.Close()

	formatter := NewFormatter(FormatTable, false)
	handler := NewCommandHandler(client, formatter)
	ctx := context.Background()

	cmds := [][]string{
		{"info", "-h"},
		{"namespaces", "-h"},
		{"types", "-h"},
		{"types", "query", "-h"},
		{"rel-types", "-h"},
		{"rel-types", "query", "-h"},
		{"objects", "-h"},
		{"objects", "query", "-h"},
		{"related", "-h"},
		{"read", "-h"},
		{"write", "-h"},
		{"history", "-h"},
		{"write-history", "-h"},
		{"sub", "-h"},
		{"sub", "create", "-h"},
		{"sub", "list", "-h"},
		{"sub", "register", "-h"},
		{"sub", "unregister", "-h"},
		{"sub", "sync", "-h"},
		{"sub", "stream", "-h"},
		{"sub", "delete", "-h"},
		{"watch", "-h"},
	}

	for _, cmd := range cmds {
		if err := handler.Execute(ctx, cmd); err != nil {
			t.Errorf("expected nil error for help on %v, got %v", cmd, err)
		}
	}
}


