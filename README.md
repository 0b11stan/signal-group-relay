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

🚧 Early design phase — nothing is implemented yet.

## License

TBD
