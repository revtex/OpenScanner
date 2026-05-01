# mosquitto/config

Static configuration for the optional bundled MQTT broker, used by
[`docker-compose.mosquitto.yml`](../../docker-compose.mosquitto.yml).

| File             | Purpose                                                                                 |
| ---------------- | --------------------------------------------------------------------------------------- |
| `mosquitto.conf` | Broker config — listener, auth, persistence, log paths. Edit and restart to apply.      |
| `passwd`         | Mosquitto password file. **Empty by default — add users before exposing the listener.** |

## Add a user

```sh
docker compose -f docker-compose.mosquitto.yml exec mosquitto \
  mosquitto_passwd -b /mosquitto/config/passwd <user> <password>
docker compose -f docker-compose.mosquitto.yml restart mosquitto
```

The first `mosquitto_passwd` invocation against the shipped empty file works without `-c`. Use `-c` only if you need to recreate it from scratch.

The `mosquitto/data/` and `mosquitto/log/` directories are git-ignored.
