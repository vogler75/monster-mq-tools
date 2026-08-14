# MonsterMQ Edge HMI Dashboards

This directory contains standalone, web-based HMI (Human-Machine Interface) applications and SCADA dashboards designed to run directly on the **MonsterMQ Edge Broker** (e.g. Siemens WinCC Unified Comfort Panels, industrial PCs, or Raspberry Pi nodes).

---

## 📂 Directory Structure

```
hmi/
├── README.md           # Documentation and deployment guide
└── example/            # Ready-to-use industrial HMI dashboard
    └── index.html      # Self-contained HMI application (HTML/CSS/JS)
```

---

## 🚀 Example HMI Dashboard (`hmi/example/`)

The included [`example/`](example/) dashboard is a fully functional industrial web interface demonstrating:
- **Live Telemetry Monitoring**: Real-time topic updates (`telemetry/#`) for temperature, pressure, and motor speed with responsive status indicators.
- **Dynamic Trend Charts**: Time-series telemetry visualization powered by Chart.js.
- **Historical Data Exploration**: Server-side historical archive queries (`archivedMessages`).
- **Interactive Process Controls**: Motor RPM setpoint adjustment, manual toggle commands, and emergency stop actions (`publish` mutation with QoS & retain flags).
- **Direct MQTT Publisher**: Ad-hoc topic payload publishing console for operational testing.
- **Modern Industrial Dark UI**: High-contrast, touch-optimized slate color theme conforming to industrial control standards.
- **Real-Time Streaming**: Pure WebSockets using the modern **`graphql-transport-ws`** subprotocol.

---

## 📦 Deploying to MonsterMQ with `mmq`

The `mmq` command-line tool allows you to upload local directories or ZIP packages directly to a running MonsterMQ broker. When passing a directory, `mmq` automatically packages the files into an in-memory ZIP bundle and deploys it via GraphQL.

### 1. Upload & Deploy Example HMI

From the `hmi/` directory:
```bash
mmq --url http://localhost:4000/graphql importHmiZip example
```

Or from the repository root:
```bash
mmq --url http://localhost:4000/graphql importHmiZip hmi/example
```

### 2. Deploy and Designate as Main Dashboard (`--main`)

To set the dashboard as the primary HMI accessible at root `/hmi/`:
```bash
mmq --url http://localhost:4000/graphql importHmiZip example --main
```

### 3. Deploy to Remote or Authenticated Brokers

```bash
# Connect to a remote Edge broker on port 4001
mmq --host 192.168.1.50 --port 4001 importHmiZip example --main

# Deploy with JWT authentication
mmq --url https://secure-broker.local:4000/graphql --token "<your-jwt-token>" importHmiZip example
```

---

## 🌐 Accessing the Deployed Dashboard

Once imported, the HMI is instantly served by MonsterMQ's built-in web server:

- **Named Dashboard Route**: `http://<broker-host>:4000/hmi/example/`
- **Primary Dashboard Route** (if deployed with `--main`): `http://<broker-host>:4000/hmi/`

---

## 🛠️ Managing Deployed HMIs

You can manage all hosted HMI dashboards using `mmq`:

```bash
# List all deployed HMI dashboards
mmq hmi list

# Export a deployed dashboard as a ZIP archive
mmq exportHmiZip example example_backup.zip

# Export and extract directly to a local directory
mmq exportHmiZip example ./extracted_hmi/ --unzip

# Delete and remove a deployed HMI
mmq hmi remove example
```

---

## ⚙️ Development Guidelines for Custom HMIs

When building custom HMIs to host in MonsterMQ Edge:

1. **Self-Contained Packages**: Keep each HMI in its own folder (e.g. `hmi/<name>/`) with `index.html` as the entrypoint.
2. **Relative Endpoints**: Use `/graphql` for HTTP requests and `location.host + '/graphql'` for WebSocket connections so the HMI works regardless of hostname or port.
3. **WebSocket Subprotocol**: Always specify the **`graphql-transport-ws`** subprotocol (`new WebSocket(wsUrl, 'graphql-transport-ws')`) and handle lifecycle events (`connection_init`, `subscribe`, `next`, and `ping`/`pong`).
4. **Publish Mutation Schema**: Use the `PublishInput` schema:
   ```javascript
   const input = {
     topic: "factory/line1/setpoint",
     payload: JSON.stringify({ speed: 1500 }),
     format: "JSON",
     qos: 1,
     retained: false
   };
   ```
