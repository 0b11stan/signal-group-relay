# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`signal2signal` is a bridge that relays messages between several Signal groups
so a community larger than Signal's ~1000-member group limit can behave like a
single group. A dedicated bot account sits in every linked group and re-posts
each message to the others, prefixed with the origin group. See
`README.md` for the full design, planned features and caveats.

## Status

Early development, built step by step with the user: don't jump ahead to
later features without asking.

- Language: Go (module `github.com/0b11stan/signal2signal`).
- Signal backend: signal-cli-rest-api in `json-rpc` mode (`docker-compose.yml`),
  exposed on `http://localhost:8081` because port 8080 is taken on the dev
  machine. Incoming messages are streamed over the
  `/v1/receive/{number}` websocket.
- Layout: `cmd/signal2signal` (entry point), `internal/signalapi` (API client
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
- **Privacy:** the bot can read every message. Don't log message contents,
  phone numbers or other personal data unless needed, and keep the Signal
  account data out of the repository.
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
nix-shell --run 'go build -o bin/signal2signal ./cmd/signal2signal'
nix-shell --run 'go vet ./...'
SIGNAL_NUMBER=+33... ./bin/signal2signal -groups   # list groups
SIGNAL_NUMBER=+33... ./bin/signal2signal           # print incoming messages
```
