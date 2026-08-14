---
name: monstermq-graphql
description: >
  Comprehensive reference and developer guide for querying, publishing, reading history, and subscribing to real-time topic data via MonsterMQ's GraphQL interface (Edge & Main brokers). Use this skill when building web HMIs, telemetry dashboards, analytics tools, or frontend data integrations.
---

# MonsterMQ Data GraphQL Skill Guide

This skill provides complete schemas, query patterns, and JavaScript code examples for fetching, streaming, publishing, and archiving telemetry and topic data from **MonsterMQ Broker** (Full/Main and Edge nodes) via GraphQL.

---

## 1. GraphQL Endpoint Architecture & Connection

MonsterMQ exposes a unified GraphQL endpoint for both HTTP queries/mutations and WebSocket real-time subscriptions.

- **HTTP Endpoint (Queries & Mutations)**: `POST http://<broker-host>:<port>/graphql`
- **WebSocket Endpoint (Subscriptions)**: `ws://<broker-host>:<port>/graphql` (Subprotocol: `graphql-transport-ws`)
- **Headers**: 
  - `Content-Type: application/json`
  - `Authorization: Bearer <jwt-token>` (if authentication is enabled)

> [!IMPORTANT]
> **WebSocket Protocol Requirement**:
> MonsterMQ uses the modern **`graphql-transport-ws`** subprotocol. Do **NOT** use the legacy, deprecated `graphql-ws` (`subscriptions-transport-ws`) protocol.
> - Subprotocol string: `'graphql-transport-ws'`
> - Subscribe message: `type: "subscribe"` (do **not** use `"start"`)
> - Inbound data message: `type: "next"` (do **not** use `"data"`)
> - Heartbeat messages: handle `type: "ping"` by responding with `type: "pong"`

### Core Payload Formats (`DataFormat` Enum)
Many queries, mutations, and subscriptions accept an optional `format: DataFormat = JSON` argument:
- `JSON`: Parses payload string as JSON (or passes standard JSON string).
- `TEXT`: Handles payload as plain UTF-8 text string.
- `BINARY`: Handles payload as Base64-encoded binary string.

---

## 2. Topic Value Inspection & Search (`Query`)

### 2.1 Get Current Value of a Single Topic (`currentValue`)
Fetch the latest stored value for an exact topic from the LastValueStore.

```graphql
query GetTopicCurrentValue($topic: String!, $format: DataFormat = JSON, $archiveGroup: String = "Default") {
  currentValue(topic: $topic, format: $format, archiveGroup: $archiveGroup) {
    topic
    payload
    format
    timestamp
    qos
    messageExpiryInterval
    contentType
    responseTopic
    payloadFormatIndicator
    userProperties {
      key
      value
    }
  }
}
```

### 2.2 Get Current Values by Topic Filter (`currentValues`)
Fetch current values across multiple topics matching MQTT wildcards (e.g., `sensors/+/temperature`, `factory/#`).

```graphql
query GetCurrentValuesFilter($topicFilter: String!, $format: DataFormat = JSON, $limit: Int = 100, $archiveGroup: String = "Default") {
  currentValues(topicFilter: $topicFilter, format: $format, limit: $limit, archiveGroup: $archiveGroup) {
    topic
    payload
    format
    timestamp
    qos
    contentType
    userProperties {
      key
      value
    }
  }
}
```

### 2.3 Fetch Retained Messages (`retainedMessage` & `retainedMessages`)
Query messages explicitly published with `retained: true`.

```graphql
# Single topic retained message
query GetRetainedMessage($topic: String!, $format: DataFormat = JSON) {
  retainedMessage(topic: $topic, format: $format) {
    topic
    payload
    format
    timestamp
    qos
    contentType
    userProperties {
      key
      value
    }
  }
}

# Filter retained messages
query GetRetainedMessages($topicFilter: String, $format: DataFormat = JSON, $limit: Int = 100) {
  retainedMessages(topicFilter: $topicFilter, format: $format, limit: $limit) {
    topic
    payload
    format
    timestamp
    qos
    contentType
  }
}
```

### 2.4 Browse Topic Hierarchy Level-by-Level (`browseTopics`)
Browse children nodes and leaf values under a topic prefix.

```graphql
query BrowseTopicTree($topic: String!, $archiveGroup: String = "Default") {
  browseTopics(topic: $topic, archiveGroup: $archiveGroup) {
    name
    isLeaf
    value(format: JSON) {
      topic
      payload
      timestamp
      qos
    }
  }
}
```

### 2.5 Search Active Topics (`searchTopics`)
Search active topic paths matching wildcard patterns (`*`, `%`, `#`) across archive groups.

```graphql
query SearchTopics($pattern: String!, $limit: Int = 100, $archiveGroup: String = "Default") {
  searchTopics(pattern: $pattern, limit: $limit, archiveGroup: $archiveGroup)
}
```

---

## 3. Data Publishing & Writing (`Mutation`)

### 3.1 Publish Single Message Payload (`publish`)
Publish a message to a topic with QoS and retain flags.

```graphql
mutation PublishMessage($input: PublishInput!) {
  publish(input: $input) {
    success
    topic
    timestamp
    error
  }
}
```

#### `PublishInput` Schema Definition:
- **`topic`** (`String!`): Exact MQTT topic name (no wildcards).
- **`payload`** (`String!`): Message payload string (JSON string, plain text, or Base64 binary).
- **`format`** (`DataFormat`): `JSON` (default), `TEXT`, or `BINARY`.
- **`qos`** (`Int`): MQTT QoS level (`0`, `1`, or `2`, default: `0`).
- **`retained`** (`Boolean`): Retain flag (`true` or `false`, default: `false`).

#### `PublishResult` Response Schema:
- **`success`** (`Boolean!`): `true` if published successfully.
- **`topic`** (`String!`): Published topic name.
- **`timestamp`** (`Long!`): Epoch millisecond timestamp of publication.
- **`error`** (`String`): Error details if failed, or `null` on success.

**Variables Example**:
```json
{
  "input": {
    "topic": "sensors/temperature/room1",
    "payload": "{\"value\": 23.4, \"unit\": \"C\"}",
    "format": "JSON",
    "qos": 1,
    "retained": true
  }
}
```

### 3.2 Batch Publish Messages (`publishBatch`)
Publish multiple telemetry readings in a single HTTP request.

```graphql
mutation PublishBatchMessages($inputs: [PublishInput!]!) {
  publishBatch(inputs: $inputs) {
    success
    topic
    timestamp
    error
  }
}
```

**Variables Example**:
```json
{
  "inputs": [
    {
      "topic": "line1/motor/speed",
      "payload": "{\"rpm\": 1450}",
      "format": "JSON",
      "qos": 0,
      "retained": false
    },
    {
      "topic": "line1/motor/temp",
      "payload": "{\"temp\": 65.2}",
      "format": "JSON",
      "qos": 1,
      "retained": true
    }
  ]
}
```

---

## 4. Archive Groups & Historical Data

### 4.1 Inspect Deployed Archive Groups (`archiveGroups`)
List all configured archive groups, enabled status, storage types, retention, and database connections.

```graphql
query ListArchiveGroups {
  archiveGroups {
    name
    enabled
    deployed
    deploymentId
    topicFilter
    retainedOnly
    lastValType
    archiveType
    databaseConnectionName
    payloadFormat
    lastValRetention
    archiveRetention
    purgeInterval
    queueType
    queueSize
    bulkSize
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
  $format: DataFormat = JSON
  $limit: Int = 100
  $archiveGroup: String = "Default"
  $includeTopic: Boolean = true
) {
  archivedMessages(
    topicFilter: $topicFilter
    startTime: $startTime
    endTime: $endTime
    format: $format
    limit: $limit
    archiveGroup: $archiveGroup
    includeTopic: $includeTopic
  ) {
    topic
    payload
    format
    timestamp
    qos
    clientId
    contentType
    userProperties {
      key
      value
    }
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
  "startTime": "2026-08-14T00:00:00Z",
  "endTime": "2026-08-14T12:00:00Z",
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
        ["2026-08-14T10:00:00Z", 22.4, 25.1],
        ["2026-08-14T10:05:00Z", 23.1, 26.0]
      ],
      "rowCount": 2
    }
  }
}
```

---

## 6. Real-Time WebSocket Subscriptions (`Subscription`)

Stream live topic updates to dashboards using the **`graphql-transport-ws`** protocol.

### 6.1 Protocol Lifecycle Overview
1. **Connect**: Connect to `ws://<host>:<port>/graphql` requesting subprotocol `'graphql-transport-ws'`.
2. **Init**: Client sends `{"type": "connection_init", "payload": { ... }}`.
3. **Ack**: Server replies `{"type": "connection_ack"}`.
4. **Subscribe**: Client sends `{"id": "1", "type": "subscribe", "payload": { "query": "...", "variables": { ... } }}`.
5. **Streaming**: Server sends frames as `{"id": "1", "type": "next", "payload": { "data": { ... } } }`.
6. **Keepalive**: Handle `{"type": "ping"}` by returning `{"type": "pong"}`.

### 6.2 Subscribe to Topic Updates (`topicUpdates`)
```graphql
subscription OnTopicUpdate($filters: [String!]!, $format: DataFormat = JSON) {
  topicUpdates(topicFilters: $filters, format: $format) {
    topic
    payload
    format
    timestamp
    qos
    retained
    clientId
  }
}
```

### 6.3 Bulk Topic Updates (`topicUpdatesBulk`)
Batch real-time updates for high-frequency topics into single WebSocket frames.

```graphql
subscription OnTopicUpdateBulk($filters: [String!]!, $format: DataFormat = JSON, $timeoutMs: Int = 1000, $maxSize: Int = 100) {
  topicUpdatesBulk(topicFilters: $filters, format: $format, timeoutMs: $timeoutMs, maxSize: $maxSize) {
    count
    timestamp
    updates {
      topic
      payload
      format
      timestamp
      qos
      retained
      clientId
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

// Publish Helper
async function publishTopic(topic, payload, { qos = 0, retained = false, format = 'JSON' } = {}) {
  const payloadStr = typeof payload === 'object' ? JSON.stringify(payload) : String(payload);
  const query = `
    mutation Publish($input: PublishInput!) {
      publish(input: $input) {
        success
        topic
        timestamp
        error
      }
    }
  `;
  const data = await graphqlFetch(query, {
    input: { topic, payload: payloadStr, format, qos, retained }
  });
  if (!data.publish.success) {
    throw new Error(data.publish.error || 'Failed to publish message');
  }
  return data.publish;
}

// Fetch Current Value Helper
async function fetchCurrentValue(topic, archiveGroup = "Default") {
  const query = `
    query GetVal($topic: String!, $group: String!) {
      currentValue(topic: $topic, format: JSON, archiveGroup: $group) {
        topic
        payload
        format
        timestamp
        qos
      }
    }
  `;
  const data = await graphqlFetch(query, { topic, group: archiveGroup });
  return data.currentValue;
}
```

### 7.2 WebSocket Subscription Manager (`graphql-transport-ws`)
```javascript
function subscribeToBrokerTopics(topicFilters, onMessageCallback, { format = 'JSON', token = null } = {}) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${location.host}/graphql`;
  
  // Explicitly specify 'graphql-transport-ws' subprotocol
  const socket = new WebSocket(wsUrl, 'graphql-transport-ws');

  socket.onopen = () => {
    const initPayload = token ? { authorization: `Bearer ${token}` } : {};
    socket.send(JSON.stringify({ type: 'connection_init', payload: initPayload }));
  };

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);

      if (msg.type === 'connection_ack') {
        // Send subscribe message with type 'subscribe' (NOT 'start')
        socket.send(JSON.stringify({
          id: 'sub-1',
          type: 'subscribe',
          payload: {
            query: `
              subscription LiveData($filters: [String!]!, $format: DataFormat) {
                topicUpdates(topicFilters: $filters, format: $format) {
                  topic
                  payload
                  format
                  timestamp
                  qos
                  retained
                  clientId
                }
              }
            `,
            variables: { filters: topicFilters, format }
          }
        }));
      } else if (msg.type === 'next' && msg.payload && msg.payload.data) {
        // Inbound data frame arrives under type 'next' (NOT 'data')
        onMessageCallback(msg.payload.data.topicUpdates);
      } else if (msg.type === 'ping') {
        // Respond to heartbeat ping
        socket.send(JSON.stringify({ type: 'pong' }));
      } else if (msg.type === 'error') {
        console.error('Subscription error:', msg.payload);
      }
    } catch (err) {
      console.error('WebSocket parse error:', err);
    }
  };

  socket.onerror = (err) => {
    console.error('WebSocket connection error:', err);
  };

  return socket;
}
```
