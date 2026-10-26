# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

`signal2signal` is a bridge that relays messages between several Signal groups
so a community larger than Signal's ~1000-member group limit can behave like a
single group. A dedicated bot account sits in every linked group and re-posts
each message to the others with the sender's name and origin group. See
`README.md` for the full design, planned features and caveats.

## Status

Early design phase: no code, language or Signal backend has been chosen yet.
Candidate backends are signal-cli (JSON-RPC daemon mode) and
signal-cli-rest-api. Don't pick a language, framework or backend without
asking first.

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
- Build, test and lint commands will be added here once the stack is chosen.
