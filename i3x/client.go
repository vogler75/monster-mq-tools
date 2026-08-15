package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// ClientConfig holds configuration for communicating with an i3x server.
type ClientConfig struct {
	BaseURL   string
	ClientID  string
	Token     string
	APIKey    string
	Headers   map[string]string
	Timeout   time.Duration
	Insecure  bool
	Verbose   bool
	UserAgent string
}

// Client is an HTTP client for the i3x 1.0 REST API.
type Client struct {
	cfg        ClientConfig
	httpClient *http.Client
	apiBase    string
}

// NewClient creates a new i3x Client from the provided configuration.
func NewClient(cfg ClientConfig) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.i3x.dev"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "i3x-cli/1.0"
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	// Normalize apiBase to point to the /v1 prefix
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	httpClient := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		apiBase:    baseURL,
	}
}

// Config returns a copy of current client configuration.
func (c *Client) Config() ClientConfig {
	return c.cfg
}

// SetBaseURL updates the target base URL.
func (c *Client) SetBaseURL(u string) {
	c.cfg.BaseURL = u
	baseURL := strings.TrimRight(u, "/")
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}
	c.apiBase = baseURL
}

// SetClientID updates the default client ID.
func (c *Client) SetClientID(id string) {
	c.cfg.ClientID = id
}

// SetToken updates the Bearer auth token.
func (c *Client) SetToken(token string) {
	c.cfg.Token = token
}

// SetAPIKey updates the API Key.
func (c *Client) SetAPIKey(key string) {
	c.cfg.APIKey = key
}

// SetHeader sets or removes a custom header.
func (c *Client) SetHeader(key, value string) {
	if c.cfg.Headers == nil {
		c.cfg.Headers = make(map[string]string)
	}
	if value == "" {
		delete(c.cfg.Headers, key)
	} else {
		c.cfg.Headers[key] = value
	}
}

// doRequest performs an HTTP request and decodes the JSON response or handles errors.
func (c *Client) doRequest(ctx context.Context, method, path string, queryParams url.Values, reqBody interface{}, out interface{}) error {
	fullURL := c.apiBase + path
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	var bodyReader io.Reader
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	for k, v := range c.cfg.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	if c.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "--> %s %s\n", method, fullURL)
		if len(bodyBytes) > 0 {
			fmt.Fprintf(os.Stderr, "    Body: %s\n", string(bodyBytes))
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	if c.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "<-- %d %s (%v)\n", resp.StatusCode, resp.Status, duration)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if c.cfg.Verbose && len(respBytes) > 0 {
		fmt.Fprintf(os.Stderr, "    Response: %s\n", string(respBytes))
	}

	// Check for HTTP errors (4xx, 5xx)
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBytes, &errResp); err == nil && (errResp.ResponseDetail.Title != "" || errResp.ResponseDetail.Detail != "") {
			return errResp.ResponseDetail
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w (raw response: %s)", err, string(respBytes))
		}
	}

	return nil
}

// -------------------------------------------------------------
// 1. Info & Health
// -------------------------------------------------------------

// GetInfo calls GET /v1/info
func (c *Client) GetInfo(ctx context.Context) (*ServerInfo, error) {
	var resp SuccessResponse[ServerInfo]
	if err := c.doRequest(ctx, http.MethodGet, "/info", nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

// -------------------------------------------------------------
// 2. Explore (Namespaces, Object Types, Relationship Types, Objects)
// -------------------------------------------------------------

// GetNamespaces calls GET /v1/namespaces
func (c *Client) GetNamespaces(ctx context.Context) ([]Namespace, error) {
	var resp SuccessResponse[[]Namespace]
	if err := c.doRequest(ctx, http.MethodGet, "/namespaces", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// GetObjectTypes calls GET /v1/objecttypes
func (c *Client) GetObjectTypes(ctx context.Context, namespaceURI string) ([]ObjectTypeResponse, error) {
	q := url.Values{}
	if namespaceURI != "" {
		q.Set("namespaceUri", namespaceURI)
	}
	var resp SuccessResponse[[]ObjectTypeResponse]
	if err := c.doRequest(ctx, http.MethodGet, "/objecttypes", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// QueryObjectTypes calls POST /v1/objecttypes/query
func (c *Client) QueryObjectTypes(ctx context.Context, elementIDs []string) (*BulkResponse[ObjectTypeResponse], error) {
	req := GetObjectTypesRequest{ElementIDs: elementIDs}
	var resp BulkResponse[ObjectTypeResponse]
	if err := c.doRequest(ctx, http.MethodPost, "/objecttypes/query", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRelationshipTypes calls GET /v1/relationshiptypes
func (c *Client) GetRelationshipTypes(ctx context.Context, namespaceURI string) ([]RelationshipType, error) {
	q := url.Values{}
	if namespaceURI != "" {
		q.Set("namespaceUri", namespaceURI)
	}
	var resp SuccessResponse[[]RelationshipType]
	if err := c.doRequest(ctx, http.MethodGet, "/relationshiptypes", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// QueryRelationshipTypes calls POST /v1/relationshiptypes/query
func (c *Client) QueryRelationshipTypes(ctx context.Context, elementIDs []string) (*BulkResponse[RelationshipType], error) {
	req := GetRelationshipTypesRequest{ElementIDs: elementIDs}
	var resp BulkResponse[RelationshipType]
	if err := c.doRequest(ctx, http.MethodPost, "/relationshiptypes/query", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetObjects calls GET /v1/objects
func (c *Client) GetObjects(ctx context.Context, typeElementID string, includeMetadata bool, root *bool) ([]ObjectInstanceResponse, error) {
	q := url.Values{}
	if typeElementID != "" {
		q.Set("typeElementId", typeElementID)
	}
	if includeMetadata {
		q.Set("includeMetadata", "true")
	}
	if root != nil {
		q.Set("root", strconv.FormatBool(*root))
	}
	var resp SuccessResponse[[]ObjectInstanceResponse]
	if err := c.doRequest(ctx, http.MethodGet, "/objects", q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Result, nil
}

// ListObjects calls POST /v1/objects/list
func (c *Client) ListObjects(ctx context.Context, elementIDs []string, includeMetadata bool) (*BulkResponse[ObjectInstanceResponse], error) {
	req := GetObjectsRequest{
		ElementIDs:      elementIDs,
		IncludeMetadata: includeMetadata,
	}
	var resp BulkResponse[ObjectInstanceResponse]
	if err := c.doRequest(ctx, http.MethodPost, "/objects/list", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryRelatedObjects calls POST /v1/objects/related
func (c *Client) QueryRelatedObjects(ctx context.Context, elementIDs []string, relationshipType string, includeMetadata bool) (*BulkResponse[[]RelatedObjectResult], error) {
	req := GetRelatedObjectsRequest{
		ElementIDs:      elementIDs,
		IncludeMetadata: includeMetadata,
	}
	if relationshipType != "" {
		req.RelationshipType = &relationshipType
	}
	var resp BulkResponse[[]RelatedObjectResult]
	if err := c.doRequest(ctx, http.MethodPost, "/objects/related", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// -------------------------------------------------------------
// 3. Values (Query Last Known & Update Current)
// -------------------------------------------------------------

// QueryLastKnownValues calls POST /v1/objects/value
func (c *Client) QueryLastKnownValues(ctx context.Context, elementIDs []string, maxDepth int) (*BulkResponse[CurrentValueResult], error) {
	req := GetObjectValueRequest{
		ElementIDs: elementIDs,
		MaxDepth:   maxDepth,
	}
	var resp BulkResponse[CurrentValueResult]
	if err := c.doRequest(ctx, http.MethodPost, "/objects/value", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateObjectValues calls PUT /v1/objects/value
func (c *Client) UpdateObjectValues(ctx context.Context, updates []ValueUpdateItem) (*BulkResponse[interface{}], error) {
	req := UpdateValueRequest{
		Updates: updates,
	}
	var resp BulkResponse[interface{}]
	if err := c.doRequest(ctx, http.MethodPut, "/objects/value", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// -------------------------------------------------------------
// 4. History (Query & Update Historical Values)
// -------------------------------------------------------------

// QueryHistoricalValues calls POST /v1/objects/history
func (c *Client) QueryHistoricalValues(ctx context.Context, elementIDs []string, startTime, endTime string, maxDepth int) (*BulkResponse[HistoricalValueResult], error) {
	req := GetObjectHistoryRequest{
		ElementIDs: elementIDs,
		StartTime:  startTime,
		EndTime:    endTime,
		MaxDepth:   maxDepth,
	}
	var resp BulkResponse[HistoricalValueResult]
	if err := c.doRequest(ctx, http.MethodPost, "/objects/history", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateObjectHistory calls PUT /v1/objects/history
func (c *Client) UpdateObjectHistory(ctx context.Context, updates []HistoryUpdateItem) (*BulkResponse[interface{}], error) {
	req := UpdateHistoryRequest{
		Updates: updates,
	}
	var resp BulkResponse[interface{}]
	if err := c.doRequest(ctx, http.MethodPut, "/objects/history", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// -------------------------------------------------------------
// 5. Subscriptions
// -------------------------------------------------------------

// CreateSubscription calls POST /v1/subscriptions
func (c *Client) CreateSubscription(ctx context.Context, clientID, displayName string) (*CreateSubscriptionResponse, error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := CreateSubscriptionRequest{
		ClientID: clientID,
	}
	if displayName != "" {
		req.DisplayName = &displayName
	}
	var resp SuccessResponse[CreateSubscriptionResponse]
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp.Result, nil
}

// RegisterMonitoredItems calls POST /v1/subscriptions/register
func (c *Client) RegisterMonitoredItems(ctx context.Context, clientID, subscriptionID string, elementIDs []string, maxDepth *int) (*BulkResponse[interface{}], error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := RegisterMonitoredItemsRequest{
		ClientID:       clientID,
		SubscriptionID: subscriptionID,
		ElementIDs:     elementIDs,
		MaxDepth:       maxDepth,
	}
	var resp BulkResponse[interface{}]
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions/register", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UnregisterMonitoredItems calls POST /v1/subscriptions/unregister
func (c *Client) UnregisterMonitoredItems(ctx context.Context, clientID, subscriptionID string, elementIDs []string) (*BulkResponse[interface{}], error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := UnregisterMonitoredItemsRequest{
		ClientID:       clientID,
		SubscriptionID: subscriptionID,
		ElementIDs:     elementIDs,
	}
	var resp BulkResponse[interface{}]
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions/unregister", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SyncSubscription calls POST /v1/subscriptions/sync
func (c *Client) SyncSubscription(ctx context.Context, clientID, subscriptionID string, lastSeq *int) ([]SyncBatch, error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := SyncRequest{
		ClientID:           clientID,
		SubscriptionID:     subscriptionID,
		LastSequenceNumber: lastSeq,
	}
	var raw json.RawMessage
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions/sync", nil, req, &raw); err != nil {
		return nil, err
	}
	return ParseSyncBatches(raw)
}

// ParseSyncBatches parses any variant of sync/stream payload into []SyncBatch.
// It flexibly handles single objects, flat arrays, batch arrays, and wrapped envelopes.
func ParseSyncBatches(data []byte) ([]SyncBatch, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	// 1. Direct array of SyncBatch: [{"sequenceNumber": 1, "updates": [...]}]
	var batches []SyncBatch
	if err := json.Unmarshal(data, &batches); err == nil && len(batches) > 0 && len(batches[0].Updates) > 0 {
		return batches, nil
	}

	// 2. Direct flat array of SyncUpdateEntry: [{"elementId": "...", "value": ...}]
	var entries []SyncUpdateEntry
	if err := json.Unmarshal(data, &entries); err == nil && len(entries) > 0 && entries[0].ElementID != "" {
		return []SyncBatch{{SequenceNumber: 1, Updates: entries}}, nil
	}

	// 3. Direct single SyncBatch object: {"sequenceNumber": 1, "updates": [...]}
	var batch SyncBatch
	if err := json.Unmarshal(data, &batch); err == nil && (len(batch.Updates) > 0 || batch.SequenceNumber > 0) {
		return []SyncBatch{batch}, nil
	}

	// 4. Direct single SyncUpdateEntry object: {"elementId": "...", "value": ...}
	var entry SyncUpdateEntry
	if err := json.Unmarshal(data, &entry); err == nil && entry.ElementID != "" {
		return []SyncBatch{{SequenceNumber: 1, Updates: []SyncUpdateEntry{entry}}}, nil
	}

	// 5. Envelope inspection (e.g. {"success": true, "result": ...} or {"results": [...]})
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Results json.RawMessage `json:"results"`
		Updates json.RawMessage `json:"updates"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		if len(envelope.Updates) > 0 {
			var updateList []SyncUpdateEntry
			if err := json.Unmarshal(envelope.Updates, &updateList); err == nil && len(updateList) > 0 {
				return []SyncBatch{{SequenceNumber: 1, Updates: updateList}}, nil
			}
		}
		if len(envelope.Result) > 0 {
			resBatches, resErr := ParseSyncBatches(envelope.Result)
			if resErr == nil && len(resBatches) > 0 {
				return resBatches, nil
			}
		}
		if len(envelope.Results) > 0 {
			var bulkItems []BulkResultItem[SyncUpdateEntry]
			if err := json.Unmarshal(envelope.Results, &bulkItems); err == nil && len(bulkItems) > 0 {
				var items []SyncUpdateEntry
				for _, it := range bulkItems {
					if it.Result != nil {
						items = append(items, *it.Result)
					}
				}
				if len(items) > 0 {
					return []SyncBatch{{SequenceNumber: 1, Updates: items}}, nil
				}
			}
			resBatches, resErr := ParseSyncBatches(envelope.Results)
			if resErr == nil && len(resBatches) > 0 {
				return resBatches, nil
			}
		}
	}

	return nil, fmt.Errorf("unrecognized sync/stream payload format: %s", string(data))
}

// ListSubscriptions calls POST /v1/subscriptions/list
func (c *Client) ListSubscriptions(ctx context.Context, clientID string, subscriptionIDs []string) (*BulkResponse[SubscriptionDetail], error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := ListSubscriptionsRequest{
		ClientID:        clientID,
		SubscriptionIDs: subscriptionIDs,
	}
	var resp BulkResponse[SubscriptionDetail]
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions/list", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteSubscriptions calls POST /v1/subscriptions/delete
func (c *Client) DeleteSubscriptions(ctx context.Context, clientID string, subscriptionIDs []string) (*BulkResponse[interface{}], error) {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}
	req := DeleteSubscriptionsRequest{
		ClientID:        clientID,
		SubscriptionIDs: subscriptionIDs,
	}
	var resp BulkResponse[interface{}]
	if err := c.doRequest(ctx, http.MethodPost, "/subscriptions/delete", nil, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StreamSubscription opens an SSE connection via POST /v1/subscriptions/stream
// (or GET fallback if server requires GET).
func (c *Client) StreamSubscription(ctx context.Context, clientID, subscriptionID string, handler func(event SSEEvent) error) error {
	if clientID == "" {
		clientID = c.cfg.ClientID
	}
	if clientID == "" {
		clientID = defaultClientID()
	}

	reqBody := StreamRequest{
		ClientID:       clientID,
		SubscriptionID: subscriptionID,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal stream request: %w", err)
	}

	// Include query parameters for servers/gateways that extract them from the URL
	q := url.Values{}
	q.Set("clientId", clientID)
	q.Set("subscriptionId", subscriptionID)
	fullURL := c.apiBase + "/subscriptions/stream?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create stream request: %w", err)
	}

	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", c.cfg.APIKey)
	}
	for k, v := range c.cfg.Headers {
		req.Header.Set(k, v)
	}

	// Use an un-timed HTTP client for streaming
	streamClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   0,
	}

	if c.cfg.Verbose {
		fmt.Fprintf(os.Stderr, "--> POST %s (SSE Stream)\n", fullURL)
		fmt.Fprintf(os.Stderr, "    Body: %s\n", string(bodyBytes))
	}

	resp, err := streamClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream connection failed: %w", err)
	}
	defer resp.Body.Close()

	// If POST is not allowed (HTTP 405), fallback to GET
	if resp.StatusCode == http.StatusMethodNotAllowed {
		resp.Body.Close()
		getReq, getErr := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
		if getErr == nil {
			getReq.Header.Set("User-Agent", c.cfg.UserAgent)
			getReq.Header.Set("Accept", "text/event-stream")
			getReq.Header.Set("Cache-Control", "no-cache")
			getReq.Header.Set("Connection", "keep-alive")
			if c.cfg.Token != "" {
				getReq.Header.Set("Authorization", "Bearer "+c.cfg.Token)
			}
			if c.cfg.APIKey != "" {
				getReq.Header.Set("X-API-Key", c.cfg.APIKey)
			}
			for k, v := range c.cfg.Headers {
				getReq.Header.Set(k, v)
			}
			if c.cfg.Verbose {
				fmt.Fprintf(os.Stderr, "--> GET %s (SSE Stream fallback)\n", fullURL)
			}
			resp, err = streamClient.Do(getReq)
			if err != nil {
				return fmt.Errorf("stream GET connection failed: %w", err)
			}
			defer resp.Body.Close()
		}
	}

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		var errResp ErrorResponse
		if err := json.Unmarshal(respBytes, &errResp); err == nil && (errResp.ResponseDetail.Title != "" || errResp.ResponseDetail.Detail != "") {
			return errResp.ResponseDetail
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	reader := bufio.NewReader(resp.Body)
	var currentEvent SSEEvent
	var dataBuffer strings.Builder

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading SSE stream: %w", err)
		}

		line = strings.TrimRight(line, "\r\n")

		// Empty line dispatches the event
		if line == "" {
			if dataBuffer.Len() > 0 || currentEvent.Event != "" {
				currentEvent.Data = dataBuffer.String()
				if err := handler(currentEvent); err != nil {
					return err
				}
				currentEvent = SSEEvent{}
				dataBuffer.Reset()
			}
			continue
		}

		// Comment line (heartbeat / ping)
		if strings.HasPrefix(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		field := parts[0]
		value := ""
		if len(parts) > 1 {
			value = strings.TrimPrefix(parts[1], " ")
		}

		switch field {
		case "event":
			currentEvent.Event = value
		case "data":
			if dataBuffer.Len() > 0 {
				dataBuffer.WriteString("\n")
			}
			dataBuffer.WriteString(value)
		case "id":
			currentEvent.ID = value
		case "retry":
			if n, err := strconv.Atoi(value); err == nil {
				currentEvent.Retry = n
			}
		}
	}
}

func defaultClientID() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		h = "i3x-cli"
	}
	// Clean hostname to keep it alphanumeric
	var clean strings.Builder
	for _, r := range h {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			clean.WriteRune(r)
		}
	}
	name := clean.String()
	if name == "" {
		name = "i3x-cli"
	}
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano()%1000000)
}
