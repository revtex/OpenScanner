# Trunk Recorder MQTT Integration

OpenScanner can subscribe to a [trunk-recorder MQTT status plugin](https://github.com/taclane/trunk-recorder-mqtt-status) feed to power a live admin dashboard covering control-channel decode rates, recorders, active calls, system tables, unit affiliation, and trunking-message debugging.

> **What this is *not*** — this integration does **not** replace call ingestion. Recorded audio still flows through `dirmonitor` (or direct `/api/v1/calls` upload) the way it always has. MQTT only adds the live operational view.

## Contents

- [Architecture](#architecture)
- [Install the plugin in trunk-recorder](#install-the-plugin-in-trunk-recorder)
- [Wire it to OpenScanner](#wire-it-to-openscanner)
- [Bundled mosquitto broker](#bundled-mosquitto-broker)
- [Multiple trunk-recorders on one broker](#multiple-trunk-recorders-on-one-broker)
- [Troubleshooting](#troubleshooting)

## Architecture

```
trunk-recorder ── publishes ──▶  MQTT broker  ◀── subscribes ──  OpenScanner
   (mqtt_status_plugin)         (mosquitto / NanoMQ /             (in-process Go subscriber,
                                 EMQX / HiveMQ / …)                eclipse/paho.golang)
```

OpenScanner runs the MQTT subscriber inside its main Go process — there is no separate sidecar. Each configured TR instance opens one supervised connection to the broker, subscribes to the topics defined below, and routes parsed frames to the admin WebSocket so the **Dashboards → Trunk Recorder** view updates live. Snapshot state is held in-memory only; nothing from the MQTT feed is persisted to the database in v1.

## Install the plugin in trunk-recorder

Build trunk-recorder with the MQTT status plugin enabled (see the [upstream README](https://github.com/taclane/trunk-recorder-mqtt-status) for build instructions). Then add a `plugins` block to your trunk-recorder `config.json`:

```jsonc
{
  // …your usual systems/sources/etc.
  "plugins": [
    {
      "name": "MQTT Status",
      "library": "libmqtt_status_plugin.so",
      "broker": "tcp://mosquitto:1883",
      "topic":         "trunk-recorder",
      "unit_topic":    "trunk-recorder/units",
      "message_topic": "trunk-recorder/messages",
      "instance_id":   "tr-headend-1",
      "client_id":     "tr-headend-1",
      "username": "tr",
      "password": "redacted",
      "qos": 0
    }
  ]
}
```

Recommended fields:

| Field           | Notes                                                                                                                                           |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `broker`        | `tcp://host:1883` for plaintext, `tls://host:8883` for TLS. Hostname must be reachable from the trunk-recorder host.                            |
| `topic`         | Base topic for status frames (`<topic>/rates`, `<topic>/recorders`, `<topic>/calls_active`, `<topic>/system`, `<topic>/config`, …).             |
| `unit_topic`    | Optional. Per-unit affiliation events publish under `<unit_topic>/<shortname>/(on\|off\|join\|location\|call\|end\|data\|ackresp\|ans_req)`.    |
| `message_topic` | Optional. Trunking control-channel messages publish under `<message_topic>/<shortname>/messages` — useful for debugging.                        |
| `instance_id`   | Stable identifier. OpenScanner uses it to drop misrouted frames when several trunk-recorders share one broker. **Set this for every TR.**       |
| `qos`           | `0` is the right answer for a live dashboard. `1` adds broker-side retransmits; `2` is rarely worth the latency.                                |

> ⚠️ The `audio` topic is **never** subscribed by OpenScanner. Audio still travels via `dirmonitor` or `/api/v1/calls`.

## Wire it to OpenScanner

1. Sign in to OpenScanner as an admin.
2. Open **Admin → Options** and confirm the **Trunk Recorder MQTT** integration is enabled (`trMqttEnabled = true`). It is off by default; flip it on once and save.
3. Open **Admin → Dashboards → Trunk Recorder → Instances** and click **Add instance**. Fill in:

| Field            | Notes                                                                                                       |
| ---------------- | ----------------------------------------------------------------------------------------------------------- |
| Label            | Free-text display name (e.g. `Headend Site 1`).                                                             |
| Instance ID      | Must match the TR plugin's `instance_id`. Frames from other instances on the same broker are ignored.       |
| Broker URL       | `tcp://…`, `tls://…`, `ws://…`, or `wss://…`. The OpenScanner side reuses the supervised reconnect loop.    |
| Base topic       | The plugin's `topic` value.                                                                                 |
| Unit topic       | Optional — must match `unit_topic` if set.                                                                  |
| Message topic    | Optional — must match `message_topic` if set.                                                               |
| Username / Pwd   | Broker credentials. Passwords are encrypted at rest with the OpenScanner encryption key (`enc::` prefix).   |
| QoS              | `0`–`2`. Match the plugin.                                                                                  |
| TLS skip verify  | Only check this for self-signed brokers in lab setups.                                                      |
| Enabled          | Toggle without deleting.                                                                                    |

4. Click **Test** to verify OpenScanner can reach the broker (CONNECT only, no subscriptions). Save once it succeeds.
5. Switch to the **Dashboard** sub-tab. Within a few seconds you should see decode rate, recorders, and any active calls. **Units** and **Messages** populate as trunking traffic arrives.

The instance row's status badge reflects the live MQTT connection: `connected` (green), `disconnected` (yellow), or `error` (red, hover for the most recent broker error).

## Bundled mosquitto broker

The repository's `docker-compose.openscanner.yml` ships with an opt-in `mosquitto` service behind the `mqtt` Compose profile. It is **not** started by default.

```sh
docker compose --profile mqtt up -d
```

On first start the entrypoint generates `./mosquitto/config/mosquitto.conf` with:

- listener on port `1883`,
- `allow_anonymous false`,
- `password_file /mosquitto/config/passwd` (initially empty),
- persistence enabled at `./mosquitto/data/`.

The published port mapping defaults to `127.0.0.1:1883:1883` — the broker is loopback-only until you change it. To expose it to your LAN, edit the `ports:` block in `docker-compose.openscanner.yml` to `"0.0.0.0:1883:1883"` (and update firewall rules accordingly).

Add a user:

```sh
docker compose exec mosquitto \
  mosquitto_passwd /mosquitto/config/passwd tr
docker compose restart mosquitto
```

(The first `mosquitto_passwd` invocation against an empty file may need the `-c` flag — `mosquitto_passwd -c /mosquitto/config/passwd tr` — to create it.)

Use that username/password in both the trunk-recorder plugin config and the OpenScanner instance form.

## Multiple trunk-recorders on one broker

Two patterns work cleanly:

1. **Disjoint base topics** — give each TR a unique `topic` (e.g. `tr-site-a`, `tr-site-b`). OpenScanner gets one instance row per TR with its own base topic. The broker doesn't care; you get cheap isolation.
2. **Shared base topic + `instance_id`** — every TR publishes to the same `topic` but sets a unique `instance_id`. OpenScanner subscribes once per row and drops frames whose `instance_id` doesn't match the configured row. Slightly more chatter on the wire but keeps the topic tree compact.

Pick whichever matches your operational style. Always set `instance_id` — it costs nothing and protects against misrouted retained messages.

## Troubleshooting

**"Test connection" returns "auth error"**

- Username or password wrong, or missing entry in `mosquitto/config/passwd`.
- For mosquitto specifically, check `./mosquitto/log/mosquitto.log` for the rejected client.

**Dashboard says "connected" but no data flows**

- Plugin's `topic` doesn't match the OpenScanner row's **Base topic**. They must be identical.
- `instance_id` mismatch — OpenScanner is dropping every frame because the plugin is publishing under a different ID.
- Run `mosquitto_sub -h <broker> -t '<base>/#' -v` to confirm the plugin is actually publishing.

**Retained messages don't show up after reconnect**

- The plugin retains `system`, `systems`, `config`, and `plugin_status`. If yours doesn't, check the upstream plugin version — older builds didn't set the retain flag on these.
- NanoMQ has historically had quirks around retained-message delivery to late subscribers; mosquitto and EMQX work out of the box.

**Oversized packet errors in broker logs**

- The trunk-recorder plugin can publish large `systems` / `config` payloads on big radio systems. Mosquitto's default `max_packet_size` (no limit) is fine; NanoMQ may need `mqtt.max_packet_size` raised.

**OpenScanner shows "processing lag" warning**

- The internal MQTT message queue is filling faster than the dashboard reducer can drain it. Usually means a busy TR with QoS 1/2 over a slow link. Drop the plugin's `qos` to `0`, or simplify the dashboard view (close Units/Messages tabs).

**The Trunk Recorder tab is missing from Dashboards**

- Check **Admin → Options → Trunk Recorder MQTT** is enabled. The kill-switch returns 404 from the REST endpoints when off, which hides the dashboard entirely.
