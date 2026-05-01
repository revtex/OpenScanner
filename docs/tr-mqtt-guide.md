# Trunk Recorder MQTT Integration

OpenScanner can subscribe to a [trunk-recorder MQTT status plugin](https://github.com/taclane/trunk-recorder-mqtt-status) feed to power a live admin dashboard covering control-channel decode rates, recorders, active calls, system tables, unit affiliation, and trunking-message debugging.

> **What this is _not_** — this integration does **not** replace call ingestion. Recorded audio still flows through `dirmonitor` (or direct `/api/v1/calls` upload) the way it always has. MQTT only adds the live operational view.

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
      "topic": "trunk-recorder",
      "unit_topic": "trunk-recorder/units",
      "message_topic": "trunk-recorder/messages",
      "instance_id": "tr-headend-1",
      "client_id": "tr-headend-1",
      "username": "tr",
      "password": "redacted",
      "qos": 0,
    },
  ],
}
```

Recommended fields:

| Field           | Notes                                                                                                                                        |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `broker`        | `tcp://host:1883` for plaintext, `tls://host:8883` for TLS. Hostname must be reachable from the trunk-recorder host.                         |
| `topic`         | Base topic for status frames (`<topic>/rates`, `<topic>/recorders`, `<topic>/calls_active`, `<topic>/system`, `<topic>/config`, …).          |
| `unit_topic`    | Optional. Per-unit affiliation events publish under `<unit_topic>/<shortname>/(on\|off\|join\|location\|call\|end\|data\|ackresp\|ans_req)`. |
| `message_topic` | Optional. Trunking control-channel messages publish under `<message_topic>/<shortname>/messages` — useful for debugging.                     |
| `instance_id`   | Stable identifier. OpenScanner uses it to drop misrouted frames when several trunk-recorders share one broker. **Set this for every TR.**    |
| `qos`           | `0` is the right answer for a live dashboard. `1` adds broker-side retransmits; `2` is rarely worth the latency.                             |

> ⚠️ The `audio` topic is **never** subscribed by OpenScanner. Audio still travels via `dirmonitor` or `/api/v1/calls`.

## Wire it to OpenScanner

1. Sign in to OpenScanner as an admin.
2. Open **Admin → Options** and confirm the **Trunk Recorder MQTT** integration is enabled (`trMqttEnabled = true`). It is off by default; flip it on once and save.
3. Open **Admin → Dashboards → Trunk Recorder → Instances** and click **Add instance**. Fill in:

| Field           | Notes                                                                                                     |
| --------------- | --------------------------------------------------------------------------------------------------------- |
| Label           | Free-text display name (e.g. `Headend Site 1`).                                                           |
| Instance ID     | Must match the TR plugin's `instance_id`. Frames from other instances on the same broker are ignored.     |
| Broker URL      | `tcp://…`, `tls://…`, `ws://…`, or `wss://…`. The OpenScanner side reuses the supervised reconnect loop.  |
| Base topic      | The plugin's `topic` value.                                                                               |
| Unit topic      | Optional — must match `unit_topic` if set.                                                                |
| Message topic   | Optional — must match `message_topic` if set.                                                             |
| Username / Pwd  | Broker credentials. Passwords are encrypted at rest with the OpenScanner encryption key (`enc::` prefix). |
| QoS             | `0`–`2`. Match the plugin.                                                                                |
| TLS skip verify | Only check this for self-signed brokers in lab setups.                                                    |
| Enabled         | Toggle without deleting.                                                                                  |

4. Click **Test** to verify OpenScanner can reach the broker (CONNECT only, no subscriptions). Save once it succeeds.
5. Switch to the **Dashboard** sub-tab. Within a few seconds you should see decode rate, recorders, and any active calls. **Units** and **Messages** populate as trunking traffic arrives.

The instance row's status badge reflects the live MQTT connection: `connected` (green), `disconnected` (yellow), or `error` (red, hover for the most recent broker error).

## Bundled mosquitto broker

The repository ships two opt-in ways to run a local MQTT broker. Pick whichever fits your operational style — they produce an equivalent broker, just with different config-management ergonomics.

### Option A — auto-bootstrapping profile (zero-touch)

`docker-compose.openscanner.yml` includes a `mosquitto` service behind the `mqtt` Compose profile. It is **not** started by default.

```sh
docker compose --profile mqtt up -d
```

On first start the container's entrypoint generates `./mosquitto/config/mosquitto.conf` with:

- listener on port `1883`,
- `allow_anonymous false`,
- `password_file /mosquitto/config/passwd`,
- persistence enabled at `./mosquitto/data/`.

After the first boot the files are yours to edit — the entrypoint only writes them when they don't already exist. The `passwd` file is **not** auto-created; you must run `mosquitto_passwd` inside the container before mosquitto will accept clients (see [Adding users](#adding-users-both-options) below).

### Option B — committed static config (explicit)

The [`mosquitto/`](../mosquitto/) folder ships a self-contained compose project that runs the upstream `eclipse-mosquitto:2` image with its default entrypoint and bind-mounts the **committed** files under [`mosquitto/config/`](../mosquitto/config/). Use it when you'd rather track the broker config in git than rely on container-side bootstrap.

```sh
cd mosquitto
docker compose up -d
```

The broker listens on `127.0.0.1:1883` by default. Logs stream to stdout — view them with:

```sh
docker compose logs -f mosquitto
```

Edit [`mosquitto/config/mosquitto.conf`](../mosquitto/config/mosquitto.conf) directly and `docker compose restart mosquitto` to apply changes. The `passwd` file is **not** committed — create it inside the container (see [Adding users](#adding-users-both-options) below) so it has the correct UID 1883 ownership and `0700` perms that mosquitto 2.x requires.

#### Files and folders

| Path                              | Purpose                                                                                                                            |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `mosquitto/docker-compose.yml`    | The standalone compose project. Run from inside `mosquitto/`.                                                                      |
| `mosquitto/config/mosquitto.conf` | Listener, auth, persistence, log destination. Logs to stdout by default so no log bind-mount is needed.                            |
| `mosquitto/config/passwd`         | Mosquitto password file. **Not committed** — created inside the container by `mosquitto_passwd` and gitignored.                    |
| `mosquitto/data/`                 | Persistent broker state (retained messages, subscriptions). Auto-created on first run — owned by container UID `1883`. Gitignored. |

### Adding users (both options)

The shipped `mosquitto.conf` boots with `allow_anonymous true` so the broker comes up cleanly on a fresh checkout (avoiding the chicken-and-egg where it would refuse to start because the `passwd` file doesn't exist yet). Lock it down once on first deployment:

1. **Hand the runtime folders to the in-container `mosquitto` user (UID 1883).** On a fresh host checkout `config/` and `data/` are owned by your shell user, but the broker runs as UID 1883 inside the container, so it can't create the password file or write persistence:

   ```sh
   cd mosquitto                         # Option B only; skip for Option A
   sudo chown -R 1883:1883 config data
   ```

   Under Option A the auto-bootstrap creates `data/` with the right owner already, but `config/` may still be host-owned if you've edited `mosquitto.conf` from the host. You'll need `sudo` (or a shared group) to edit `mosquitto.conf` from the host afterwards.

2. **Start the broker** — `docker compose up -d` (or, for Option A, `docker compose --profile mqtt up -d`).

3. **Create the first user from inside the container.** The `-c` flag creates the file. `--user mosquitto` drops the exec session to UID 1883 so the file is owned by the in-container `mosquitto` user with `0700` perms (which mosquitto 2.x requires) — without it `docker compose exec` runs as root and the broker can't read the file it just made:

   ```sh
   docker compose exec --user mosquitto mosquitto \
     mosquitto_passwd -c -b /mosquitto/config/passwd tr '<password>'
   ```

4. **Lock the broker down.** Edit `mosquitto/config/mosquitto.conf` (with `sudo` after the chown): comment out `allow_anonymous true`, uncomment `allow_anonymous false` and `password_file /mosquitto/config/passwd`.

5. **Restart**: `docker compose restart mosquitto`.

For **additional users** (with the broker already running and `passwd` already present), drop the `-c` flag so you don't overwrite the file. Keep `--user mosquitto` so the modified file stays owned by UID 1883:

```sh
docker compose exec --user mosquitto mosquitto \
  mosquitto_passwd -b /mosquitto/config/passwd <user> '<password>'
docker compose restart mosquitto
```

Drop the `-b` flag if you want to be prompted instead of passing the password on the command line.

### Exposing the broker beyond loopback

Both compose files publish `127.0.0.1:1883:1883` by default — the broker is loopback-only until you change it. To expose it to your LAN, edit the `ports:` block to `"0.0.0.0:1883:1883"` (and update firewall rules accordingly). Make sure you've added users first.

Use the configured username/password in both the trunk-recorder plugin config and the OpenScanner instance form.

## Multiple trunk-recorders on one broker

Two patterns work cleanly:

1. **Disjoint base topics** — give each TR a unique `topic` (e.g. `tr-site-a`, `tr-site-b`). OpenScanner gets one instance row per TR with its own base topic. The broker doesn't care; you get cheap isolation.
2. **Shared base topic + `instance_id`** — every TR publishes to the same `topic` but sets a unique `instance_id`. OpenScanner subscribes once per row and drops frames whose `instance_id` doesn't match the configured row. Slightly more chatter on the wire but keeps the topic tree compact.

Pick whichever matches your operational style. Always set `instance_id` — it costs nothing and protects against misrouted retained messages.

## Troubleshooting

**"Test connection" returns "auth error"**

- Username or password wrong, or missing entry in `mosquitto/config/passwd`.
- Check the broker logs with `docker compose logs mosquitto` (Option B) or `docker compose logs mosquitto` from the OpenScanner stack (Option A) — rejected clients are logged at connect time.

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
