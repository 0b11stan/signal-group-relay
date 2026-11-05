# signal-group-relay

A bridge that relays messages between multiple Signal groups, so a community
larger than Signal's group size limit (~1000 members) can talk as if it were a
single group.

## The problem

Signal groups are capped at roughly 1000 members. A community of 3000 people
therefore has to be split across several groups (e.g. *Community A*, *B* and
*C*), and moderators end up manually copy-pasting announcements and
discussions from one group to the others.

## The idea

`signal-group-relay` runs a dedicated Signal account (a "bridge bot") that is a
member of every group in the community. Whenever a message is posted in one
group, the bot re-posts it to all the other linked groups, prefixed with the
origin group:

```
[Community A] Meeting is moved to 8pm tonight!
```

```
 ┌────────────┐        ┌────────────┐        ┌────────────┐
 │  Group A   │◀──────▶│   bridge   │◀──────▶│  Group B   │
 └────────────┘        │    bot     │        └────────────┘
                       │            │        ┌────────────┐
                       │            │◀──────▶│  Group C   │
                       └────────────┘        └────────────┘
```

## Planned features

- **Message relay** — text messages from any linked group are forwarded to
  all the others.
- **Origin** — relayed messages show the group they came from. The author's
  name and number are not relayed.
- **Attachments** — images, files and voice notes are relayed too.
- **Replies & quotes** — a reply to a relayed message is shown as a reply in
  the other groups as well, when possible.
- **Reactions** *(optional)* — mirror emoji reactions across groups.
- **Loop prevention** — the bot never re-relays its own messages.
- **Configurable bridges** — define which groups are linked together, and
  optionally one-way relays (e.g. an announcements-only group).
- **Opt-out / filtering** — skip messages with a given prefix (e.g. `!local`)
  so a message can stay inside a single group.

## Architecture (draft)

- **Signal backend:** [signal-cli](https://github.com/AsamK/signal-cli)
  (run in JSON-RPC / daemon mode) or
  [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api).
  The bridge registers or links its own phone number and joins every group.
- **Bridge service:** a small long-running process that subscribes to incoming
  messages, looks up which bridge the source group belongs to, and sends the
  message to the other groups.
- **State:** a small local store (e.g. SQLite) mapping relayed message
  timestamps across groups, used for replies, reactions and edits.
- **Deployment:** a container (Docker / Podman) with a persistent volume for
  the Signal account data.

## Configuration (draft)

```yaml
signal:
  account: "+33600000000"

bridges:
  - name: community
    groups:
      - id: "base64-group-id-A"
        label: "Community A"
      - id: "base64-group-id-B"
        label: "Community B"
      - id: "base64-group-id-C"
        label: "Community C"
```

## Getting started

At this stage the program relays text messages and images between the
linked groups.

### Requirements

- Docker with Compose (or Podman with `podman compose`)
- Go 1.26+ (on Nix/NixOS, `nix-shell` gives you Go from `shell.nix`)
- The Signal app on your phone

### 1. Start the Signal API

```sh
docker compose up -d
```

This runs [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)
in `json-rpc` mode on `http://localhost:8080` (bound to localhost only: the
API has no authentication). Account data is kept in the `signal-cli-data`
Docker volume. Its logs are discarded (`logging: driver: none`): signal-cli
prints every message it receives, with names and phone numbers. To debug the
API, comment that out temporarily, and recreate the container afterwards so
the logs are deleted.

Check it is up:

```sh
curl http://localhost:8080/v1/about
```

### 2. Link the API to your Signal account

The API joins your account as a *linked device*, like Signal Desktop.

1. Open <http://localhost:8080/v1/qrcodelink?device_name=signal-group-relay> in a
   browser. It shows a QR code.
2. On your phone: Signal → Settings → Linked devices → Link new device, and
   scan the code.
3. Check the account is known to the API:

   ```sh
   curl http://localhost:8080/v1/accounts
   ```

4. Restart the API so the daemon picks up the new account:

   ```sh
   docker compose restart
   ```

> **Note:** linked this way, the bridge can read *all* your conversations.
> That's fine for experimenting. For production, register a dedicated number
> for the bot instead.

### 3. List your groups

```sh
export SIGNAL_NUMBER=+33600000000   # your number, international format

go run ./cmd/signal-group-relay -groups
```

### 4. Relay messages

Put the ids of the groups to link (the `id=` value printed by `-groups`) in a
file, one per line, at least two. Blank lines and lines starting with `#` are
ignored. The program refuses to start with fewer than two:

```
# groups.txt
abc...=
def...=
```

```sh
go run ./cmd/signal-group-relay -linked-groups groups.txt
```

The program refuses to start if an id isn't one of your account's groups. Then
every message posted in a linked group is posted to all the other linked
groups (Ctrl+C to stop). The program does nothing without `-linked-groups`
(or `-groups`): it prints its usage.

The bridge follows a **no-log** policy: it never logs message contents,
authors, phone numbers, group names or ids, nor any per-message activity.
Logs only contain startup information and errors (e.g. `relay: send:
unexpected status 400 Bad Request`), with the API's error details left out
since they can quote phone numbers.

Since the bridge is a linked device of your account, the relayed copies appear
as sent by you:

- messages written by other members are prefixed with the origin group, e.g.
  `[Community A] hello`. The author's name and phone number are never relayed;
- messages you write yourself are sent as they are.

Images are relayed with their caption. Not relayed yet: other attachments (files, voice notes, videos), replies, mentions, reactions and edits.

Use `-api` if the API is not on `http://localhost:8080`.

## Running on a dedicated account

Linked to your personal account, every copy appears as sent by you. For the
real community, give the bridge its own Signal account:

1. **Get a number** that can receive one SMS or voice call. Don't use your
   personal one: registering a number here removes it from the Signal app.
2. **Register it** against the running API (solve the captcha at
   <https://signalcaptchas.org/registration/generate.html> and copy the
   `signalcaptchas://...` token):

   ```sh
   BOT=+33...
   curl -X POST localhost:8080/v1/register/$BOT -H 'Content-Type: application/json' \
        -d '{"captcha": "signalcaptchas://..."}'          # add "use_voice": true for a call
   curl -X POST localhost:8080/v1/register/$BOT/verify/123456   # the received code
   curl -X PUT  localhost:8080/v1/profiles/$BOT -H 'Content-Type: application/json' \
        -d '{"name": "Community bridge"}'
   docker compose restart signal-api
   curl localhost:8080/v1/accounts
   ```

3. **Add the bot** to each group (it takes one member slot per group), then
   `SIGNAL_NUMBER=$BOT go run ./cmd/signal-group-relay -groups` and write
   `groups.txt` from its output.
4. **Run the bridge** in the container, next to the API:

   ```sh
   SIGNAL_NUMBER=$BOT docker compose --profile bridge up -d --build
   ```

   It reconnects by itself if the API restarts (messages received while
   disconnected are not replayed). A plain `docker compose up -d` still only
   starts the API.

Back up the `signal-cli-data` volume: it holds the bot's keys.

## Caveats

- The bridge account must be a member (ideally admin) of every group.
- Relayed messages are sent by the bot, not by the original author: members
  can't reply privately to an author through the relayed copy.
- Signal end-to-end encryption still applies between each member and the bot,
  but the bot itself can read every message. Run it on infrastructure you
  trust and tell your community it exists.
- Signal may rate-limit accounts that send a large volume of messages; very
  busy communities may need throttling.

## Status

🚧 Early development: text messages and images are relayed between the
linked groups. See *Planned features* for what is still missing.

## License

TBD
