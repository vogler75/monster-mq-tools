---
name: monstermq-hmi-builder
description: >
  Guide for creating, editing, and packaging HTML/JS/CSS HMI apps and dashboards served by the MonsterMQ Edge broker.
  Use this skill whenever building industrial HMIs, SCADA screens, live topic charts, or telemetry dashboards hosted at /hmi/.
  Trigger on mentions of "create HMI", "build dashboard", "HMI screen", "live chart", "topic history", "WinCC", "web HMI",
  or when generating HTML apps for MonsterMQ Edge.
---

# MonsterMQ Edge HMI Builder Skill

This skill provides comprehensive instructions, architecture patterns, UI design guidelines, and copy-paste boilerplate code for creating standalone, HTML/JS-based HMIs and dashboards hosted directly by the MonsterMQ Edge broker (e.g. running on Siemens WinCC Unified Comfort Panels, industrial PCs, or Raspberry Pi).

---

## 1. HMI Hosting Architecture & Routing

- **Base Directory**: `./data/hmi/`
- **Dashboard Structure**: Every dashboard app lives in its own subdirectory under `./data/hmi/<dashboardname>/` containing `index.html` and static assets.
- **Main Dashboard**: Served at `http://<broker>:4000/hmi/` (alias for the designated primary dashboard app, default: `main`).
- **Named Dashboards**: Served at `http://<broker>:4000/hmi/<dashboardname>/`.

---

## 2. Deploying & Testing HMIs with MonsterMQ CLI (`mmq`)

The MonsterMQ CLI tool (`mmq` in `cli/bin/mmq`) provides native commands to build, package, deploy, inspect, and test HMI dashboards directly against local or remote edge brokers.

> [!TIP]
> See the companion skill [**`monstermq-cli`**](file:///Users/vogler/Workspace/monster/tools/.agents/skills/monstermq-cli/SKILL.md) for the complete CLI reference.

### 2.1 HMI Dashboard Lifecycle Commands
| Task | CLI Command |
| :--- | :--- |
| **List Dashboards** | `mmq hmis` *(or `mmq hmi list`)* |
| **Create Definition** | `mmq hmi create <name> --title "Title" --path /<name> [--main]` |
| **Deploy from Folder** | `mmq importHmiZip <folder-path> [name] [--main]` *(auto-zips in memory)* |
| **Deploy from Zip** | `mmq importHmiZip <package.zip> [name] [--main]` |
| **Export to Folder** | `mmq exportHmiZip <name> <target-dir> --unzip` *(auto-extracts files)* |
| **Export to Zip** | `mmq exportHmiZip <name> [output.zip]` |
| **Delete Dashboard** | `mmq hmi remove <name1> [name2...]` |

### 2.2 Telemetry Simulation & UI Testing Commands
When developing or testing an HMI screen, use `mmq` to discover topics and inject mock sensor data:

```bash
# 1. Discover active topics for widget binding
mmq searchTopics "factory/#"

# 2. Inspect current topic values
mmq currentValue factory/line1/temperature

# 3. Publish mock live telemetry to test gauges and charts
mmq publish factory/line1/temperature '{"temp": 24.8, "unit": "C"}'
mmq publish factory/line1/pressure '{"bar": 4.2}'
mmq publish factory/line1/speed '{"rpm": 1500}'

# 4. Check historical time-series data for chart validation
mmq archivedMessages factory/line1/temperature --last-seconds 300

# 5. Connect to remote Edge devices or WinCC panels
mmq --host 192.168.1.50 --port 4001
```

---

## 3. Broker Data Access (GraphQL API)

All HMIs communicate directly with the broker's GraphQL endpoint (`/graphql`) over HTTP (`POST /graphql`) and WebSockets (`ws://<broker>:4000/graphql` with **`graphql-transport-ws`** subprotocol).

> [!IMPORTANT]
> **WebSocket Protocol Standard (`graphql-transport-ws`)**:
> Always use the modern standard **`graphql-transport-ws`** subprotocol (`new WebSocket(wsUrl, 'graphql-transport-ws')`). Do **NOT** use the deprecated `graphql-ws` (`subscriptions-transport-ws`) protocol:
> - Client sends `{"type": "connection_init"}` on connect.
> - Server replies `{"type": "connection_ack"}`.
> - Client subscribes using `{"id": "1", "type": "subscribe", "payload": { "query": "..." }}` (do **not** use `"start"`).
> - Live data arrives in `{"id": "1", "type": "next", "payload": { "data": { ... } }}` (do **not** use `"data"`).
> - Handle server `{"type": "ping"}` by replying `{"type": "pong"}`.

### 2.1 Fetching Current Topic Values (`currentValue` & `currentValues`)
```javascript
// Fetch single topic current value
async function getTopicValue(topic, archiveGroup = "Default") {
    const query = `
        query GetTopicVal($topic: String!, $group: String!) {
            currentValue(topic: $topic, format: JSON, archiveGroup: $group) {
                topic
                payload
                format
                timestamp
                qos
            }
        }
    `;
    const res = await fetch('/graphql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, variables: { topic, group: archiveGroup } })
    });
    const result = await res.json();
    return result.data?.currentValue;
}

// Fetch multiple topics by wildcard filter (e.g. "sensors/#")
async function getTopicValues(topicFilter, limit = 100) {
    const query = `
        query GetTopicVals($filter: String!, $limit: Int!) {
            currentValues(topicFilter: $filter, format: JSON, limit: $limit) {
                topic
                payload
                format
                timestamp
                qos
            }
        }
    `;
    const res = await fetch('/graphql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, variables: { filter: topicFilter, limit } })
    });
    const result = await res.json();
    return result.data?.currentValues || [];
}
```

### 2.2 Reading Historical Messages (`archivedMessages`)
```javascript
async function getHistory(topicFilter, { startTime = null, endTime = null, limit = 100, archiveGroup = "Default" } = {}) {
    const query = `
        query GetHistory($filter: String!, $start: String, $end: String, $limit: Int!, $group: String!) {
            archivedMessages(topicFilter: $filter, startTime: $start, endTime: $end, format: JSON, limit: $limit, archiveGroup: $group) {
                topic
                payload
                format
                timestamp
                qos
                clientId
            }
        }
    `;
    const res = await fetch('/graphql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            query,
            variables: { filter: topicFilter, start: startTime, end: endTime, limit, group: archiveGroup }
        })
    });
    const result = await res.json();
    return result.data?.archivedMessages || [];
}
```

### 2.3 Publishing Commands & Control Signals (`publish`)
```javascript
async function publishControl(topic, payload, { qos = 0, retained = false, format = 'JSON' } = {}) {
    const query = `
        mutation PublishCmd($input: PublishInput!) {
            publish(input: $input) {
                success
                topic
                timestamp
                error
            }
        }
    `;
    const payloadStr = typeof payload === 'object' ? JSON.stringify(payload) : String(payload);
    const variables = {
        input: {
            topic,
            payload: payloadStr,
            format, // 'JSON', 'TEXT', or 'BINARY'
            qos,
            retained // Boolean (default: false)
        }
    };
    const res = await fetch('/graphql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query, variables })
    });
    const result = await res.json();
    if (result.errors && result.errors.length > 0) {
        throw new Error(`GraphQL Error: ${result.errors[0].message}`);
    }
    if (!result.data.publish.success) {
        throw new Error(result.data.publish.error || 'Failed to publish control message');
    }
    return result.data.publish;
}
```

### 2.4 Real-time Subscriptions (`graphql-transport-ws`)
```javascript
function subscribeTopicUpdates(topicFilters, onUpdate) {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/graphql`;
    
    // Explicitly use 'graphql-transport-ws'
    const socket = new WebSocket(wsUrl, 'graphql-transport-ws');

    socket.onopen = () => {
        socket.send(JSON.stringify({ type: 'connection_init' }));
    };

    socket.onmessage = (event) => {
        try {
            const msg = JSON.parse(event.data);
            if (msg.type === 'connection_ack') {
                // Subscribe message using type 'subscribe' (NOT 'start')
                socket.send(JSON.stringify({
                    id: '1',
                    type: 'subscribe',
                    payload: {
                        query: `
                            subscription LiveTelemetry($filters: [String!]!) {
                                topicUpdates(topicFilters: $filters, format: JSON) {
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
                        variables: { filters: topicFilters }
                    }
                }));
            } else if (msg.type === 'next' && msg.payload && msg.payload.data) {
                // Live telemetry arrives under type 'next' (NOT 'data')
                onUpdate(msg.payload.data.topicUpdates);
            } else if (msg.type === 'ping') {
                // Heartbeat response
                socket.send(JSON.stringify({ type: 'pong' }));
            }
        } catch (err) {
            console.error('WebSocket parse error:', err);
        }
    };

    return socket;
}
```

---

## 4. Industrial UI Design Tokens & Theme

To ensure HMIs feel premium, modern, and readable on industrial panels (Siemens WinCC Comfort Panels, touchscreens), follow this color palette and layout standard:

- **Background**: `#0f172a` (Slate 900)
- **Card / Surface Container**: `#1e293b` (Slate 800) with `border: 1px solid #334155`
- **Primary Text**: `#f8fafc` (Slate 50)
- **Secondary Text**: `#94a3b8` (Slate 400)
- **Accent Primary**: `#38bdf8` (Sky 400)
- **Status OK / Running**: `#22c55e` (Emerald 500)
- **Status Warning**: `#f59e0b` (Amber 500)
- **Status Alarm / Stopped**: `#ef4444` (Red 500)

---

## 5. Complete Boilerplate Dashboard Template

When creating a new HMI screen, generate a self-contained single-file HTML like the one below:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Industrial Monitoring & Control HMI</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; padding: 1.5rem; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid #334155; }
        .title { font-size: 1.5rem; font-weight: 600; color: #38bdf8; }
        .status-badge { background: #064e3b; color: #34d399; padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.875rem; font-weight: 500; }
        .status-badge.connecting { background: #78350f; color: #fbbf24; }
        .status-badge.disconnected { background: #7f1d1d; color: #fca5a5; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem; margin-bottom: 1.5rem; }
        .card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1.25rem; }
        .card-label { font-size: 0.875rem; color: #94a3b8; margin-bottom: 0.5rem; }
        .card-value { font-size: 2.25rem; font-weight: 700; color: #f8fafc; }
        .card-unit { font-size: 1rem; color: #64748b; font-weight: 400; }
        .chart-container { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1.25rem; height: 350px; margin-bottom: 1.5rem; }
        .controls-card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 1.25rem; display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
        .btn { background: #0284c7; color: white; border: none; padding: 0.6rem 1.2rem; border-radius: 6px; font-weight: 500; cursor: pointer; transition: background 0.2s; }
        .btn:hover { background: #0369a1; }
        .btn:active { transform: scale(0.98); }
        .btn-danger { background: #dc2626; }
        .btn-danger:hover { background: #b91c1c; }
        .control-input { background: #0f172a; border: 1px solid #334155; color: #f8fafc; padding: 0.5rem 0.75rem; border-radius: 6px; width: 100px; font-size: 1rem; }
    </style>
</head>
<body>
    <div class="header">
        <div class="title">Production Line A — Live Telemetry & Control</div>
        <div class="status-badge connecting" id="connStatus">CONNECTING</div>
    </div>

    <div class="grid">
        <div class="card">
            <div class="card-label">Temperature</div>
            <div class="card-value" id="tempVal">-- <span class="card-unit">°C</span></div>
        </div>
        <div class="card">
            <div class="card-label">Pressure</div>
            <div class="card-value" id="pressVal">-- <span class="card-unit">bar</span></div>
        </div>
        <div class="card">
            <div class="card-label">Motor Speed</div>
            <div class="card-value" id="speedVal">-- <span class="card-unit">RPM</span></div>
        </div>
    </div>

    <div class="chart-container">
        <canvas id="lineChart"></canvas>
    </div>

    <div class="controls-card">
        <span style="font-weight: 600; color: #94a3b8;">Setpoint Control:</span>
        <input type="number" id="targetSpeed" class="control-input" value="1500" step="50" min="0" max="3000">
        <button class="btn" onclick="applySetpoint()">Set Motor RPM</button>
        <button class="btn btn-danger" onclick="emergencyStop()">E-STOP</button>
        <span id="cmdFeedback" style="font-size: 0.875rem; color: #34d399; margin-left: auto;"></span>
    </div>

    <script>
        // Init Line Chart
        const ctx = document.getElementById('lineChart').getContext('2d');
        const chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Temperature (°C)',
                    data: [],
                    borderColor: '#38bdf8',
                    backgroundColor: 'rgba(56, 189, 248, 0.1)',
                    fill: true,
                    tension: 0.3
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { labels: { color: '#94a3b8' } } },
                scales: {
                    x: { ticks: { color: '#64748b' }, grid: { color: '#334155' } },
                    y: { ticks: { color: '#64748b' }, grid: { color: '#334155' } }
                }
            }
        });

        // GraphQL Query Helper
        async function graphqlFetch(query, variables = {}) {
            const res = await fetch('/graphql', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ query, variables })
            });
            const json = await res.json();
            if (json.errors && json.errors.length > 0) throw new Error(json.errors[0].message);
            return json.data;
        }

        // Publish Control Signals
        async function publishTopic(topic, payload, retained = false) {
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
                input: { topic, payload: payloadStr, format: 'JSON', qos: 1, retained }
            });
            if (!data.publish.success) throw new Error(data.publish.error || 'Publish failed');
            return data.publish;
        }

        async function applySetpoint() {
            const rpm = parseFloat(document.getElementById('targetSpeed').value);
            const fb = document.getElementById('cmdFeedback');
            try {
                await publishTopic('factory/line1/motor/setpoint', { speedRpm: rpm });
                fb.textContent = `✓ Setpoint ${rpm} RPM sent`;
                fb.style.color = '#34d399';
            } catch (err) {
                fb.textContent = `✗ Error: ${err.message}`;
                fb.style.color = '#ef4444';
            }
        }

        async function emergencyStop() {
            const fb = document.getElementById('cmdFeedback');
            try {
                await publishTopic('factory/line1/motor/estop', { stop: true, timestamp: Date.now() });
                fb.textContent = `⚠ E-STOP Triggered!`;
                fb.style.color = '#ef4444';
            } catch (err) {
                fb.textContent = `✗ Error: ${err.message}`;
                fb.style.color = '#ef4444';
            }
        }

        // Live Subscriptions via graphql-transport-ws protocol
        let ws = null;
        function initWebSocket() {
            const wsProtocol = location.protocol === 'https:' ? 'wss://' : 'ws:';
            const wsUrl = wsProtocol + location.host + '/graphql';
            
            // Standard graphql-transport-ws subprotocol
            ws = new WebSocket(wsUrl, 'graphql-transport-ws');

            ws.onopen = () => {
                ws.send(JSON.stringify({ type: 'connection_init' }));
            };

            ws.onmessage = (e) => {
                try {
                    const msg = JSON.parse(e.data);
                    if (msg.type === 'connection_ack') {
                        const statusBadge = document.getElementById('connStatus');
                        statusBadge.textContent = 'CONNECTED';
                        statusBadge.className = 'status-badge';

                        // Subscribe using type 'subscribe'
                        ws.send(JSON.stringify({
                            id: '1',
                            type: 'subscribe',
                            payload: {
                                query: `subscription { topicUpdates(topicFilters: ["factory/line1/#"], format: JSON) { topic payload timestamp retained } }`
                            }
                        }));
                    } else if (msg.type === 'next' && msg.payload && msg.payload.data) {
                        // Data arrives with type 'next'
                        const update = msg.payload.data.topicUpdates;
                        handleTopicUpdate(update.topic, update.payload, update.timestamp);
                    } else if (msg.type === 'ping') {
                        ws.send(JSON.stringify({ type: 'pong' }));
                    }
                } catch (err) { console.error('WS parse error', err); }
            };

            ws.onclose = () => {
                const statusBadge = document.getElementById('connStatus');
                statusBadge.textContent = 'DISCONNECTED';
                statusBadge.className = 'status-badge disconnected';
                // Auto reconnect after 3 seconds
                setTimeout(initWebSocket, 3000);
            };

            ws.onerror = () => {
                const statusBadge = document.getElementById('connStatus');
                statusBadge.textContent = 'ERROR';
                statusBadge.className = 'status-badge disconnected';
            };
        }

        function handleTopicUpdate(topic, payloadStr, timestamp) {
            try {
                const val = typeof payloadStr === 'string' ? JSON.parse(payloadStr) : payloadStr;
                const timeStr = new Date(timestamp).toLocaleTimeString();

                if (topic.endsWith('/temperature')) {
                    const temp = typeof val === 'object' ? val.value ?? val.temp : val;
                    document.getElementById('tempVal').childNodes[0].nodeValue = Number(temp).toFixed(1) + ' ';
                    
                    chart.data.labels.push(timeStr);
                    chart.data.datasets[0].data.push(temp);
                    if (chart.data.labels.length > 20) {
                        chart.data.labels.shift();
                        chart.data.datasets[0].data.shift();
                    }
                    chart.update();
                } else if (topic.endsWith('/pressure')) {
                    const press = typeof val === 'object' ? val.value ?? val.pressure : val;
                    document.getElementById('pressVal').childNodes[0].nodeValue = Number(press).toFixed(2) + ' ';
                } else if (topic.endsWith('/speed')) {
                    const spd = typeof val === 'object' ? val.value ?? val.rpm : val;
                    document.getElementById('speedVal').childNodes[0].nodeValue = Math.round(spd) + ' ';
                }
            } catch (err) { console.error('Payload parse error', err); }
        }

        // Start WebSocket
        initWebSocket();
    </script>
</body>
</html>
```
