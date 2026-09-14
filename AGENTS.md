# AGENTS.md

Go single-binary MP4 streaming server (playlist of mp4s with a web player UI). Fiber v2 + django templates (pongo2) + viper. Verification = `go build ./...` / `go vet ./...` / `go test ./...` (all pass). Unit tests live in `pkg/videoserver/*_test.go` (TotpStore, auth flow, enrollment) using fiber `app.Test()` + stub templates in `pkg/videoserver/testdata/views/` — they mirror production wiring, don't embed the real views.

## Build & run

- Dev build: `make local` → binary at `bin/go-mp4-server` (stripped, `-ldflags "-s -w"`). `make debug` → non-stripped build at repo root (`go-mp4-server`, not in `bin/`).
- `make release` cross-compiles into `bin/<os>/` (mac, linux, win, pi=armv7, arm64, mips). `make docker` builds a local image.
- Run locally (config comes from env only, no config file):
  ```
  GOMP4_VIDEO_DIR=$PWD/videos GOMP4_SERVER_PORT=30080 ./bin/go-mp4-server
  ```
  `static/` is read from cwd (`./static`), so run from the repo root. `videos/` is gitignored — drop mp4s there to test.
- Docker: multi-arch image published by CI (`.github/workflows/docker-image.yml`) to Docker Hub `doublehub/go-mp4-server` and GHCR on push/PR to `main` (needs `DOCKERHUB_TOKEN`, `MY_GITHUB_TOKEN` secrets).

## Config (viper)

Env prefix `GOMP4_`, `.` → `_` in keys (`pkg/config/config.go`):
- `GOMP4_VIDEO_DIR` — mp4 source dir. **Default is empty**; server errors without it.
- `GOMP4_SERVER_PORT` — default `3000`.
- `GOMP4_SERVER_AUTH_CONFIG_PATH` — default `./auth.json`.
- `GOMP4_SERVER_TOTP_CONFIG_PATH` — TOTP (2FA) enrollment file, default `./2fa.json`. Gitignored — secrets must never be committed.
- `GOMP4_SERVER_TOTP_ISSUER` — label shown in authenticator apps, default `go-mp4-server`.

Auth is session-based: a login page (`/login`) validates against `auth.json` (read on **every** login, so password edits apply without restart), then an optional TOTP step (`/verify`) when that user has 2FA enabled. `auth.json` is committed with default creds `admin`/`admin` — don't commit real credentials. Users self-enroll via `/settings` (QR code; `2fa.json` stores secrets). Sessions are in-memory (24h) — a restart logs everyone out. Lost authenticator: manually delete the user's entry from `2fa.json`.

## Architecture

- `main.go` — entrypoint; embeds `views/` via `//go:embed views` (templates live inside the binary, so view edits require a rebuild; the `Reload` flag does not help with the embedded FS).
- `pkg/config/config.go` — viper init via `init()`; imported blank from `pkg/videoserver` (side-effect import). Prints keys/debug output on every startup.
- `pkg/videoserver/` — `videoserver.go` (app wiring, video routes, recursive `getMp4Files` mapping to `/videos/...` served with `ByteRange: true`), `auth.go` (session middleware + login/verify/logout), `totp.go` (`TotpStore`, mutex-protected `2fa.json` access), `settings.go` (2FA enrollment UI + server-side QR PNG). `getMp4Files` walks `VIDEO.DIR` **recursively**, keeps only `*.mp4` (skips dotfiles).
- `views/` — `*.django` templates (django syntax via pongo2), partials in `views/partials/`. `static/` served at `/static`.

## Conventions

- Go 1.22.2 (`.go-version`, `go.mod`). Module name is `go-mp4-server` (not a URL path) — imports look like `go-mp4-server/pkg/...`.
- Conventional commits in git history (`feat:`, `fix:`, `ci:`, `refactor:`).
- `bin/` build artifacts are untracked but not in `.gitignore` — don't commit them.