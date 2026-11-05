# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`signal-group-relay` is a bridge that relays messages between several Signal groups
so a community larger than Signal's ~1000-member group limit can behave like a
single group. A dedicated bot account sits in every linked group and re-posts
each message to the others, prefixed with the origin group. See
`README.md` for the full design, planned features and caveats.

## Status

Early development, built step by step with the user: don't jump ahead to
later features without asking.

- Language: Go (module `github.com/0b11stan/signal-group-relay`).
- Signal backend: signal-cli-rest-api in `json-rpc` mode (`docker-compose.yml`),
  exposed on `http://localhost:8080` because port 8080 is taken on the dev
  machine. Incoming messages are streamed over the
  `/v1/receive/{number}` websocket.
- Layout: `cmd/signal-group-relay` (entry point), `internal/signalapi` (API client
  and message types).
- Ask before adding dependencies.

## Git rules

- **Never commit without explicit approval.** Before every `git commit`, show
  the proposed changes and commit message and wait for the user to say yes.
  An earlier approval does not cover later commits.
- Never push, force-push, rebase, amend or rewrite history unless explicitly
  asked to.
- The default branch is `main`.

## Design principles

- **Loop prevention is critical:** the bot must never relay its own messages,
  or the groups will flood each other.
- **Privacy / no-log:** the bot can read every message, and the server
  policy is "no-log". Never log or print message contents, authors, phone
  numbers, group names or ids, or per-message activity; errors that may be
  logged must not carry them either (watch for URLs holding the account
  number and API error bodies). Keep the Signal account data out of the
  repository.
- **Secrets:** never commit phone numbers, group IDs, account keys or real
  config files. Commit example configs only (e.g. `config.example.yaml`).
- **Rate limits:** Signal may throttle busy accounts. Keep sending
  rate-aware and handle send failures gracefully.

## Conventions

- Keep the README up to date when features or configuration change.
- Run `gofmt`, `go vet ./...` and `go build ./...` before considering a change
  done.

## Commands

Go is not installed globally on the dev machine (NixOS): run Go commands
through `nix-shell --run '...'`, which uses `shell.nix`.

```sh
docker compose up -d                       # start the Signal API
nix-shell --run 'go build -o bin/signal-group-relay ./cmd/signal-group-relay'
nix-shell --run 'go vet ./...'
SIGNAL_NUMBER=+33... ./bin/signal-group-relay -groups   # list groups
SIGNAL_NUMBER=+33... ./bin/signal-group-relay -linked-groups groups.txt   # relay
```
