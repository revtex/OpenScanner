# mosquitto/config

Static configuration for the optional bundled MQTT broker, used by
[`../docker-compose.yml`](../docker-compose.yml).

| File             | Purpose                                                                                                                                                                          |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mosquitto.conf` | Broker config — listener, auth, persistence, log destination. Edit and restart to apply.                                                                                         |
| `passwd`         | Mosquitto password file. **Not committed.** Created inside the container the first time you run `mosquitto_passwd` (see below). Gitignored so each deployment owns its own file. |

## First-run setup (anonymous → authenticated)

The shipped `mosquitto.conf` boots with `allow_anonymous true` so the
broker comes up cleanly on a fresh checkout (avoiding the
chicken-and-egg where mosquitto would refuse to start because the
`passwd` file doesn't exist yet, which then blocks `docker compose exec`).

Follow this once per deployment:

1. **Hand the runtime folders to the in-container `mosquitto` user.**
   On a fresh host checkout `config/` and `data/` are owned by your
   shell user, but the broker runs as UID 1883 inside the container.
   Without this step `mosquitto_passwd` can't create files in `config/`
   and the broker can't write persistence:

   ```sh
   sudo chown -R 1883:1883 config data
   ```

   You'll now need `sudo` (or membership in a shared group) to edit
   `mosquitto.conf` from the host — normal for a self-hosted broker.

2. **Start the broker:**

   ```sh
   docker compose up -d
   ```

3. **Create the password file from inside the container.** The `-c`
   flag creates the file. `--user mosquitto` drops the exec session to
   UID 1883 so the file is owned by the in-container `mosquitto` user
   with `0700` perms (which mosquitto 2.x requires) — without it
   `docker compose exec` runs as root and the file ends up unreadable
   by the broker:

   ```sh
   docker compose exec --user mosquitto mosquitto \
     mosquitto_passwd -c -b /mosquitto/config/passwd <user> '<password>'
   ```

4. **Lock the broker down.** Edit `mosquitto.conf` (with `sudo`) and
   swap the commented/uncommented auth lines:

   ```conf
   #allow_anonymous true
   allow_anonymous false
   password_file /mosquitto/config/passwd
   ```

5. **Restart:**

   ```sh
   docker compose restart mosquitto
   ```

The broker now requires auth. The `passwd` file lives in the
bind-mounted `config/` folder, so this bootstrap is one-time per
deployment — future container rebuilds reuse it.

## Adding more users

With the broker running and the `passwd` file already present, drop the
`-c` flag (it would otherwise overwrite the file). Keep `--user
mosquitto` so the modified file stays owned by UID 1883:

```sh
docker compose exec --user mosquitto mosquitto \
  mosquitto_passwd -b /mosquitto/config/passwd <user> <password>
docker compose restart mosquitto
```

Drop `-b` if you'd rather be prompted instead of passing the password on
the command line.

The `mosquitto/data/` directory is git-ignored. Logs go to stdout — view
them with `docker compose logs mosquitto`.
