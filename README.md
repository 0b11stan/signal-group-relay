# signal-group-relay

Relays messages between several Signal groups, so a community larger than
Signal's group size limit (~1000 members) can talk as if it were a single
group.

A bot account is a member of every linked group. When someone posts in one
group, the bot re-posts the message to all the others, prefixed with the
group it came from:

```
[Community A] Meeting is moved to 8pm tonight!
```

## What is relayed

- Text messages.
- Images (JPEG, PNG, GIF and WebP) with their caption. The format is checked
  from the file content, not from the type announced by the sender.

The author's name and phone number are never relayed: only the origin
group is shown. The bot never relays its own messages, so groups can't
flood each other.

Not relayed: other attachments (files, voice notes, videos), replies,
mentions, reactions, edits and deletions.

## Setup

Requirements: Docker with Compose, and Go 1.26+ to run the program outside
a container (`nix-shell` provides it on NixOS).

### 1. Start the Signal API

```sh
docker compose up -d
```

This starts [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)
on `http://localhost:8080`. It is only reachable from this machine, as the
API has no authentication. The account data lives in the `signal-cli-data`
volume. Back it up: it holds the bot's keys.

### 2. Register the bot's account

Use a dedicated number that can receive an SMS or a voice call, not your
personal one: registering a number removes it from the Signal app. Solve the
captcha at <https://signalcaptchas.org/registration/generate.html> and copy
the `signalcaptchas://...` token, then:

```sh
BOT=+33...
curl -X POST localhost:8080/v1/register/$BOT -H 'Content-Type: application/json' \
     -d '{"captcha": "signalcaptchas://..."}'          # add "use_voice": true for a call
curl -X POST localhost:8080/v1/register/$BOT/verify/123456   # the received code
curl -X PUT  localhost:8080/v1/profiles/$BOT -H 'Content-Type: application/json' \
     -d '{"name": "Community bridge"}'
docker compose restart signal-api
```

<details>
<summary>Testing with your personal account instead</summary>

The API can join your own account as a linked device: open
<http://localhost:8080/v1/qrcodelink?device_name=signal-group-relay>, scan
the QR code from Signal → Settings → Linked devices, then run
`docker compose restart signal-api`. The bridge can then read all your
conversations and relayed copies appear as sent by you. Messages you write
yourself are relayed without the group prefix.

</details>

### 3. Choose the groups to link

Add the bot to each group, then list its groups:

```sh
SIGNAL_NUMBER=$BOT go run ./cmd/signal-group-relay -groups
```

Write the `id=` values of the groups to link in `groups.txt` at the root of
the project, one per line.
Blank lines and lines starting with `#` are ignored:

```
# groups.txt
abc...=
def...=
```

### 4. Run the bridge

In a container next to the API:

```sh
SIGNAL_NUMBER=$BOT docker compose --profile bridge up -d --build
```

Or directly:

```sh
SIGNAL_NUMBER=$BOT go run ./cmd/signal-group-relay -linked-groups groups.txt
```

The bridge refuses to start unless `groups.txt` lists at least two of the
bot's groups. It reconnects by itself if the API restarts; messages received
while disconnected are not relayed.

## Options

| Flag             | Default                 | Description                                  |
| ---------------- | ----------------------- | -------------------------------------------- |
| `-number`        | `$SIGNAL_NUMBER`        | The bot's phone number.                      |
| `-groups`        |                         | List the bot's groups and exit.              |
| `-linked-groups` |                         | File with the ids of the groups to link.     |
| `-api`           | `http://localhost:8080` | signal-cli-rest-api URL.                     |

## Privacy

The bot can read every message in the linked groups. Run it on a machine you
trust, and tell your community it exists.

Nothing personal is logged. The bridge logs only startup information and
errors, never message contents, authors, phone numbers, group names or ids.
The Signal API's container logs are disabled, as signal-cli prints every
message it receives. To debug the API, remove `logging: driver: none` from
`docker-compose.yml` for a while, then recreate the container to delete
those logs.

## Limitations

- Relayed messages come from the bot, so members can't reply privately to the
  original author from the copy.
- Every message is sent once per other group, with no throttling. Signal may
  rate-limit the bot in very busy communities.
