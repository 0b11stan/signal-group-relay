# signal2signal

A bridge that relays messages between multiple Signal groups, so a community
larger than Signal's group size limit (~1000 members) can talk as if it were a
single group.

## The problem

Signal groups are capped at roughly 1000 members. A community of 3000 people
therefore has to be split across several groups (e.g. *Community A*, *B* and
*C*), and moderators end up manually copy-pasting announcements and
discussions from one group to the others.

## The idea

`signal2signal` runs a dedicated Signal account (a "bridge bot") that is a
member of every group in the community. Whenever a message is posted in one
group, the bot re-posts it to all the other linked groups, prefixed with the
author's name and origin group:

```
[Community A] Alice: Meeting is moved to 8pm tonight!
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
- **Attribution** — relayed messages show the original sender's display name
  and the group they came from.
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

At this stage the program only connects to Signal and prints the messages it
receives.

### Requirements

- Docker with Compose (or Podman with `podman compose`)
- Go 1.26+ (on Nix/NixOS, `nix-shell` gives you Go from `shell.nix`)
- The Signal app on your phone

### 1. Start the Signal API

```sh
docker compose up -d
```

This runs [signal-cli-rest-api](https://github.com/bbernhard/signal-cli-rest-api)
in `json-rpc` mode on `http://localhost:8081` (bound to localhost only: the
API has no authentication). Account data is kept in the `signal-cli-data`
Docker volume.

Check it is up:

```sh
curl http://localhost:8081/v1/about
```

### 2. Link the API to your Signal account

The API joins your account as a *linked device*, like Signal Desktop.

1. Open <http://localhost:8081/v1/qrcodelink?device_name=signal2signal> in a
   browser. It shows a QR code.
2. On your phone: Signal → Settings → Linked devices → Link new device, and
   scan the code.
3. Check the account is known to the API:

   ```sh
   curl http://localhost:8081/v1/accounts
   ```

4. Restart the API so the daemon picks up the new account:

   ```sh
   docker compose restart
   ```

> **Note:** linked this way, the bridge can read *all* your conversations.
> That's fine for experimenting. For production, register a dedicated number
> for the bot instead.

### 3. Read messages

```sh
export SIGNAL_NUMBER=+33600000000   # your number, international format

# List your groups and their ids
go run ./cmd/signal2signal -groups

# Print incoming messages (Ctrl+C to stop)
go run ./cmd/signal2signal
```

Send a message in one of your groups and it shows up:

```
[2026-10-26 12:30:00] group "Community A" id=abc...= | Alice: hello!
```

Messages you type on your phone also show up, tagged `(me)`.

To only watch the groups you want to link, put their ids (the `id=` value
printed by `-groups`) in a file, one per line. Blank lines and lines starting
with `#` are ignored:

```
# groups.txt
abc...=
def...=
```

```sh
go run ./cmd/signal2signal -linked-groups groups.txt
```

The program refuses to start if an id isn't one of your account's groups.

### 4. Relay messages

```sh
# See what would be sent, without sending anything
go run ./cmd/signal2signal -linked-groups groups.txt -relay -dry-run

# Really relay
go run ./cmd/signal2signal -linked-groups groups.txt -relay
```

With `-relay`, every text message posted in a linked group is posted to all the
other linked groups. Since the bridge is a linked device of your account, the
copies appear as sent by you:

- messages written by other members are prefixed with the origin group and the
  author's display name, e.g. `[Community A] Alice: hello`. Phone numbers are
  never relayed;
- messages you write yourself are sent as they are.

Not relayed yet: attachments, replies, mentions, reactions and edits.

Use `-api` if the API is not on `http://localhost:8081`.

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

🚧 Early development: the bridge can connect to Signal and print incoming
messages, but doesn't relay them yet.

## License

TBD
