---
name: monstermq-graphql
description: >
  Comprehensive reference and developer guide for querying, publishing, reading history, and subscribing to real-time topic data via MonsterMQ's GraphQL interface (Edge & Main brokers). Use this skill when building web HMIs, telemetry dashboards, analytics tools, or frontend data integrations.
---

# MonsterMQ Data GraphQL Skill Guide

This skill provides complete schemas, query patterns, and JavaScript code examples for fetching, streaming, and storing telemetry and topic data from **MonsterMQ Broker** (Full/Main and Edge nodes) via GraphQL.

---

## 1. GraphQL Endpoint Architecture & Connection

MonsterMQ exposes a unified GraphQL endpoint for both HTTP queries/mutations and WebSocket real-time subscriptions.

- **HTTP Endpoint (Queries & Mutations)**: `POST http://<broker-host>:<port>/graphql`
- **WebSocket Endpoint (Subscriptions)**: `ws://<broker-host>:<port>/graphql` (Protocol: `graphql-ws`)
- **Headers**: 
  - `Content-Type: application/json`
  - `Authorization: Bearer <jwt-token>` (if authentication is enabled)

---

## 2. Topic Value Inspection & Search

### 2.1 Get Current Value of a Single Topic
Fetch the latest/current stored value for a topic from a specific archive group.

```graphql
query GetTopicCurrentValue($topic: String!, $archiveGroup: String = "Default") {
  currentValue(topic: $topic, format: JSON, archiveGroup: $archiveGroup) {
    topic
    payload
    format
    timestamp
    qos
  }
}
```

### 2.2 Get Current Values by Topic Filter Pattern
Fetch current values across multiple topics matching wildcard filters (e.g., `factory/floor1/#`).

```graphql
query GetCurrentValuesFilter($filter: String!, $limit: Int = 100) {
  currentValues(topicFilter: $filter, format: JSON, limit: $limit) {
    topic
    payload
    timestamp
    qos
  }
}
```

### 2.3 Fetch Retained Messages
Query retained messages directly from the broker's retained store.

```graphql
query GetRetainedMessages($filter: String) {
  retainedMessages(topicFilter: $filter, limit: 100) {
    topic
    payload
    timestamp
    qos
    contentType
  }
}
```

### 2.4 Search Active Topics
Search active topic paths matching wildcard patterns across archive groups.

```graphql
query SearchTopics($pattern: String!, $archiveGroup: String = "Default") {
  searchTopics(pattern: $pattern, limit: 100, archiveGroup: $archiveGroup)
}
```

---

## 3. Data Publishing & Writing (`Mutation`)

### 3.1 Publish Single Message Payload
Publish a message to a topic with QoS and retain flags.

```graphql
mutation PublishMessage($input: PublishInput!) {
  publish(input: $input) {
    success
    topic
    timestamp
    error
    message
  }
}
```

**Variables Example**:
```json
{
  "input": {
    "topic": "sensors/temperature/room1",
    "payload": "{\"value\": 23.4, \"unit\": \"C\"}",
    "qos": 1,
    "retained": true,
    "format": "JSON"
  }
}
```
> **Note**: For retain flag compatibility across brokers, pass `"retained": true` (Main Broker standard) or `"retain": true` (Edge Broker).

### 3.2 Batch Publish Messages
Publish multiple telemetry readings in a single HTTP POST request.

```graphql
mutation PublishBatchMessages($inputs: [PublishInput!]!) {
  publishBatch(inputs: $inputs) {
    success
    topic
    timestamp
    error
    message
  }
}
```

---

## 4. Archive Groups & Historical Data

### 4.1 Inspect Deployed Archive Groups
List all configured archive groups, enabled status, storage types, and target database connections.

```graphql
query ListArchiveGroups {
  archiveGroups {
    name
    enabled
    deployed
    topicFilter
    retainedOnly
    lastValType
    archiveType
    databaseConnectionName
  }
}
```

### 4.2 Query Historical Messages (`archivedMessages`)
Retrieve historical message records within an ISO-8601 time window.

```graphql
query QueryMessageHistory(
  $topicFilter: String!
  $startTime: String
  $endTime: String
  $limit: Int = 100
  $archiveGroup: String = "Default"
) {
  archivedMessages(
    topicFilter: $topicFilter
    startTime: $startTime
    endTime: $endTime
    limit: $limit
    archiveGroup: $archiveGroup
  ) {
    topic
    payload
    format
    timestamp
    qos
    clientId
  }
}
```

### 4.3 Archive Statistics & Volume Counts (`archiveStats`)
Get daily message volume counts and the earliest recorded timestamp for an archive group.

```graphql
query GetArchiveStats($archiveGroup: String!, $startTime: String, $endTime: String) {
  archiveStats(archiveGroup: $archiveGroup, startTime: $startTime, endTime: $endTime) {
    minTimestamp
    dailyCounts {
      date
      count
    }
  }
}
```

---

## 5. Time-Series Data Aggregation (`aggregatedMessages`)

Perform server-side time-bucket aggregations over numerical metrics across multiple topics.

### 5.1 Aggregation Query Syntax
```graphql
query QueryAggregated(
  $topics: [String!]!
  $interval: AggregationInterval!
  $startTime: String!
  $endTime: String!
  $functions: [AggregationFunction!] = [AVG]
  $fields: [String!]
  $archiveGroup: String = "Default"
) {
  aggregatedMessages(
    topics: $topics
    interval: $interval
    startTime: $startTime
    endTime: $endTime
    functions: $functions
    fields: $fields
    archiveGroup: $archiveGroup
  ) {
    columns
    rows
    interval
    startTime
    endTime
    topicCount
    rowCount
  }
}
```

### 5.2 Aggregation Enums Reference
- **`AggregationInterval`**: `ONE_MINUTE`, `FIVE_MINUTES`, `FIFTEEN_MINUTES`, `ONE_HOUR`, `ONE_DAY`
- **`AggregationFunction`**: `AVG`, `MIN`, `MAX`, `COUNT`

**Variables Example**:
```json
{
  "topics": ["telemetry/sensor1", "telemetry/sensor2"],
  "interval": "FIVE_MINUTES",
  "startTime": "2026-08-13T00:00:00Z",
  "endTime": "2026-08-13T12:00:00Z",
  "functions": ["AVG", "MAX"],
  "archiveGroup": "Default"
}
```

**Tabular Response Format**:
```json
{
  "data": {
    "aggregatedMessages": {
      "columns": ["timestamp", "telemetry/sensor1_avg", "telemetry/sensor1_max"],
      "rows": [
        ["2026-08-13T10:00:00Z", 22.4, 25.1],
        ["2026-08-13T10:05:00Z", 23.1, 26.0]
      ],
      "rowCount": 2
    }
  }
}
```

---

## 6. Real-Time WebSocket Subscriptions (`Subscription`)

Stream live topic updates to dashboards using standard `graphql-ws` WebSockets.

### 6.1 Subscribe to Topic Updates (`topicUpdates`)
```graphql
subscription OnTopicUpdate($filters: [String!]!) {
  topicUpdates(topicFilters: $filters, format: JSON) {
    topic
    payload
    format
    timestamp
    qos
    retained
  }
}
```

### 6.2 Bulk Topic Updates (`topicUpdatesBulk`)
Batch real-time updates for high-frequency topics into single WebSocket frames.

```graphql
subscription OnTopicUpdateBulk($filters: [String!]!, $timeoutMs: Int = 500, $maxSize: Int = 50) {
  topicUpdatesBulk(topicFilters: $filters, format: JSON, timeoutMs: $timeoutMs, maxSize: $maxSize) {
    count
    timestamp
    updates {
      topic
      payload
      format
      timestamp
      qos
      retained
    }
  }
}
```

---

## 7. Complete JavaScript Client Boilerplate for Dashboards

### 7.1 HTTP Query & Mutation Helper
```javascript
async function graphqlFetch(query, variables = {}, token = null) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch('/graphql', {
    method: 'POST',
    headers,
    body: JSON.stringify({ query, variables })
  });

  const res = await response.json();
  if (res.errors && res.errors.length > 0) {
    throw new Error(`GraphQL Error: ${res.errors[0].message}`);
  }
  return res.data;
}

// Example Usage:
// const data = await graphqlFetch(`query { currentValue(topic: "sensors/temp") { payload } }`);
```

### 7.2 WebSocket Subscription Manager
```javascript
function subscribeToBrokerTopics(topicFilters, onMessageCallback) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${location.host}/graphql`;
  const socket = new WebSocket(wsUrl, 'graphql-ws');

  socket.onopen = () => {
    socket.send(JSON.stringify({ type: 'connection_init' }));
  };

  socket.onmessage = (event) => {
    const msg = JSON.parse(event.data);

    if (msg.type === 'connection_ack') {
      socket.send(JSON.stringify({
        id: 'sub-1',
        type: 'start',
        payload: {
          query: `
            subscription LiveData($filters: [String!]!) {
              topicUpdates(topicFilters: $filters) {
                topic
                payload
                timestamp
                qos
                retained
              }
            }
          `,
          variables: { filters: topicFilters }
        }
      }));
    } else if (msg.type === 'data' && msg.payload && msg.payload.data) {
      onMessageCallback(msg.payload.data.topicUpdates);
    }
  };

  return socket;
}
```
