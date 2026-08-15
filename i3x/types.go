package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// ServerCapabilities models the capability flags of an i3x server.
type ServerCapabilities struct {
	Query struct {
		History bool `json:"history"`
	} `json:"query"`
	Update struct {
		Current bool `json:"current"`
		History bool `json:"history"`
	} `json:"update"`
	Subscribe struct {
		Stream bool `json:"stream"`
	} `json:"subscribe"`
}

// ServerInfo models GET /info response result.
type ServerInfo struct {
	SpecVersion   string             `json:"specVersion"`
	ServerVersion *string            `json:"serverVersion,omitempty"`
	ServerName    *string            `json:"serverName,omitempty"`
	Capabilities  ServerCapabilities `json:"capabilities"`
}

// Namespace models GET /namespaces response items.
type Namespace struct {
	URI         string `json:"uri"`
	DisplayName string `json:"displayName"`
}

// ObjectTypeResponse models object type schemas.
type ObjectTypeResponse struct {
	ElementID    string                 `json:"elementId"`
	DisplayName  string                 `json:"displayName"`
	NamespaceURI string                 `json:"namespaceUri"`
	SourceTypeID string                 `json:"sourceTypeId"`
	Version      *string                `json:"version,omitempty"`
	Schema       map[string]interface{} `json:"schema"`
	Related      map[string]interface{} `json:"related,omitempty"`
}

// RelationshipType models relationship types.
type RelationshipType struct {
	ElementID      string `json:"elementId"`
	DisplayName    string `json:"displayName"`
	NamespaceURI   string `json:"namespaceUri"`
	RelationshipID string `json:"relationshipId"`
	ReverseOf      string `json:"reverseOf"`
}

// ObjectInstanceMetadata models extended metadata for an object instance.
type ObjectInstanceMetadata struct {
	TypeNamespaceURI *string                `json:"typeNamespaceUri,omitempty"`
	SourceTypeID     *string                `json:"sourceTypeId,omitempty"`
	Description      *string                `json:"description,omitempty"`
	Relationships    map[string]interface{} `json:"relationships,omitempty"`
	SchemaExtensions map[string]interface{} `json:"schemaExtensions,omitempty"`
	System           map[string]interface{} `json:"system,omitempty"`
}

// ObjectInstanceResponse models an object instance.
type ObjectInstanceResponse struct {
	ElementID     string                  `json:"elementId"`
	DisplayName   string                  `json:"displayName"`
	TypeElementID string                  `json:"typeElementId"`
	ParentID      *string                 `json:"parentId,omitempty"`
	IsComposition bool                    `json:"isComposition"`
	IsExtended    bool                    `json:"isExtended,omitempty"`
	Metadata      *ObjectInstanceMetadata `json:"metadata,omitempty"`
}

// RelatedObjectResult models a related object item.
type RelatedObjectResult struct {
	SourceRelationship string                 `json:"sourceRelationship"`
	Object             ObjectInstanceResponse `json:"object"`
}

// VQT models Value, Quality, Timestamp.
type VQT struct {
	Value     interface{} `json:"value"`
	Quality   string      `json:"quality"`
	Timestamp string      `json:"timestamp"`
}

// VQTInput models Value, Quality, Timestamp for input updates.
type VQTInput struct {
	Value     interface{} `json:"value"`
	Quality   *string     `json:"quality,omitempty"`
	Timestamp *string     `json:"timestamp,omitempty"`
}

// ValueUpdateItem models elementId and VQTInput for PUT /objects/value.
type ValueUpdateItem struct {
	ElementID string   `json:"elementId"`
	Value     VQTInput `json:"value"`
}

// HistoryUpdateItem models elementId and VQT for PUT /objects/history.
type HistoryUpdateItem struct {
	ElementID string `json:"elementId"`
	Value     VQT    `json:"value"`
}

// CurrentValueResult models result of GET/POST /objects/value.
type CurrentValueResult struct {
	IsComposition bool           `json:"isComposition"`
	Value         interface{}    `json:"value"`
	Quality       string         `json:"quality"`
	Timestamp     string         `json:"timestamp"`
	Components    map[string]VQT `json:"components,omitempty"`
}

// HistoricalValueResult models result of POST /objects/history.
type HistoricalValueResult struct {
	IsComposition bool                   `json:"isComposition"`
	Values        []VQT                  `json:"values"`
	Components    map[string]interface{} `json:"components,omitempty"`
}

// MonitoredObject models an item registered in a subscription.
type MonitoredObject struct {
	ElementID string `json:"elementId"`
	MaxDepth  *int   `json:"maxDepth,omitempty"`
}

// SubscriptionDetail models subscription information in listSubscriptions.
type SubscriptionDetail struct {
	SubscriptionID   string            `json:"subscriptionId"`
	DisplayName      *string           `json:"displayName,omitempty"`
	MonitoredObjects []MonitoredObject `json:"monitoredObjects"`
}

// CreateSubscriptionResponse models the result from POST /subscriptions.
type CreateSubscriptionResponse struct {
	ClientID       string  `json:"clientId"`
	SubscriptionID string  `json:"subscriptionId"`
	DisplayName    *string `json:"displayName,omitempty"`
}

// SyncUpdateEntry models a single update inside a SyncBatch.
type SyncUpdateEntry struct {
	ElementID string      `json:"elementId"`
	Value     interface{} `json:"value"`
	Quality   string      `json:"quality"`
	Timestamp string      `json:"timestamp"`
}

// SyncBatch models a batch of updates in syncSubscription.
type SyncBatch struct {
	SequenceNumber int               `json:"sequenceNumber"`
	Updates        []SyncUpdateEntry `json:"updates"`
}

// ErrorDetail models standard i3x error payload details.
type ErrorDetail struct {
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
}

func (e ErrorDetail) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d %s] %s", e.Status, e.Title, e.Detail)
	}
	return fmt.Sprintf("[%d %s]", e.Status, e.Title)
}

// ErrorResponse models top-level error response.
type ErrorResponse struct {
	Success        bool        `json:"success"`
	ResponseDetail ErrorDetail `json:"responseDetail"`
}

// SuccessResponse generic envelope for single result responses.
type SuccessResponse[T any] struct {
	Success bool `json:"success"`
	Result  T    `json:"result"`
}

// BulkResultItem models an individual item in a bulk operation response.
type BulkResultItem[T any] struct {
	Success        bool         `json:"success"`
	ElementID      *string      `json:"elementId,omitempty"`
	SubscriptionID *string      `json:"subscriptionId,omitempty"`
	Result         *T           `json:"result,omitempty"`
	ResponseDetail *ErrorDetail `json:"responseDetail,omitempty"`
}

// BulkResponse models a list of BulkResultItems.
type BulkResponse[T any] struct {
	Success bool                `json:"success"`
	Results []BulkResultItem[T] `json:"results"`
}

// --- Request Payloads ---

// GetObjectTypesRequest for POST /objecttypes/query
type GetObjectTypesRequest struct {
	ElementIDs []string `json:"elementIds"`
}

// GetRelationshipTypesRequest for POST /relationshiptypes/query
type GetRelationshipTypesRequest struct {
	ElementIDs []string `json:"elementIds"`
}

// GetObjectsRequest for POST /objects/list
type GetObjectsRequest struct {
	ElementIDs      []string `json:"elementIds"`
	IncludeMetadata bool     `json:"includeMetadata"`
}

// GetRelatedObjectsRequest for POST /objects/related
type GetRelatedObjectsRequest struct {
	ElementIDs       []string `json:"elementIds"`
	RelationshipType *string  `json:"relationshipType,omitempty"`
	IncludeMetadata  bool     `json:"includeMetadata"`
}

// GetObjectValueRequest for POST /objects/value
type GetObjectValueRequest struct {
	ElementIDs []string `json:"elementIds"`
	MaxDepth   int      `json:"maxDepth"`
}

// UpdateValueRequest for PUT /objects/value
type UpdateValueRequest struct {
	Updates []ValueUpdateItem `json:"updates"`
}

// GetObjectHistoryRequest for POST /objects/history
type GetObjectHistoryRequest struct {
	ElementIDs []string `json:"elementIds"`
	StartTime  string   `json:"startTime"`
	EndTime    string   `json:"endTime"`
	MaxDepth   int      `json:"maxDepth"`
}

// UpdateHistoryRequest for PUT /objects/history
type UpdateHistoryRequest struct {
	Updates []HistoryUpdateItem `json:"updates"`
}

// CreateSubscriptionRequest for POST /subscriptions
type CreateSubscriptionRequest struct {
	ClientID    string  `json:"clientId"`
	DisplayName *string `json:"displayName,omitempty"`
}

// RegisterMonitoredItemsRequest for POST /subscriptions/register
type RegisterMonitoredItemsRequest struct {
	ClientID       string   `json:"clientId"`
	SubscriptionID string   `json:"subscriptionId"`
	ElementIDs     []string `json:"elementIds"`
	MaxDepth       *int     `json:"maxDepth,omitempty"`
}

// UnregisterMonitoredItemsRequest for POST /subscriptions/unregister
type UnregisterMonitoredItemsRequest struct {
	ClientID       string   `json:"clientId"`
	SubscriptionID string   `json:"subscriptionId"`
	ElementIDs     []string `json:"elementIds"`
}

// StreamRequest for POST /subscriptions/stream
type StreamRequest struct {
	ClientID       string `json:"clientId"`
	SubscriptionID string `json:"subscriptionId"`
}

// SyncRequest for POST /subscriptions/sync
type SyncRequest struct {
	ClientID           string `json:"clientId"`
	SubscriptionID     string `json:"subscriptionId"`
	LastSequenceNumber *int   `json:"lastSequenceNumber,omitempty"`
}

// DeleteSubscriptionsRequest for POST /subscriptions/delete
type DeleteSubscriptionsRequest struct {
	ClientID        string   `json:"clientId"`
	SubscriptionIDs []string `json:"subscriptionIds"`
}

// ListSubscriptionsRequest for POST /subscriptions/list
type ListSubscriptionsRequest struct {
	ClientID        string   `json:"clientId"`
	SubscriptionIDs []string `json:"subscriptionIds"`
}

// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// ParseValueHelper parses a string argument into float, bool, int, or json object if possible.
func ParseValueHelper(raw string) interface{} {
	var val interface{}
	if err := json.Unmarshal([]byte(raw), &val); err == nil {
		return val
	}
	return raw
}

// FormatTimeRFC3339 formats time into RFC3339 string UTC.
func FormatTimeRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
