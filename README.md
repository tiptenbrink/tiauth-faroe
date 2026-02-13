## WIP: Do not yet run in production!

Monorepo for the Tiauth Faroe distribution and friends.

Includes:
- Tiauth Faroe server distribution (Go, in `tiauth/`)
- Python client library (`python/client`)
- Python user server library (`python/user-server`)
- Python sync reference server (`python/sync-reference-server`)

## Tiauth Faroe server distribution

The Tiauth Faroe server distribution is a _distribution_ of the [Faroe project](https://github.com/faroedev/faroe) with opinionated defaults.

It has the following features:
- E-mail sending delegated to a user server (Python backend) via HTTP
- SQLite database for user storage
- Configuration via `.env` file (pass a custom path with `--env-file`)
- A `/command` endpoint on a separate listener (`127.0.0.2`) for management commands
- Session expiration configuration (`FAROE_SESSION_EXPIRATION`)
- CORS origin configuration (`FAROE_CORS_ALLOW_ORIGIN`)

### Architecture

The Go server communicates with a user server via HTTP on `127.0.0.2:<user-server-port>` (default 12790). The user server handles:
- User action invocations (create/update/delete user) via `POST /invoke`
- Email sending and token storage via `POST /email`

Management commands (e.g. reset) are served on a separate listener on `127.0.0.2:<command-port>` (default 12771), accessible only from localhost.

### Configuration

Environment variables (set in `.env` file or OS environment):

| Variable | Default | Description |
|---|---|---|
| `FAROE_DB_PATH` | `./db.sqlite` | Path to SQLite database |
| `FAROE_PORT` | `12770` | HTTP server port |
| `FAROE_USER_SERVER_PORT` | `12790` | Port where the user server listens (on 127.0.0.2) |
| `FAROE_COMMAND_PORT` | `12771` | Port for management commands (on 127.0.0.2) |
| `FAROE_SESSION_EXPIRATION` | `2160h` (90 days) | Session expiration duration |
| `FAROE_CORS_ALLOW_ORIGIN` | (empty) | Allowed CORS origin |

CLI flags:

| Flag | Description |
|---|---|
| `--env-file` | Path to environment file (default: `.env`) |
| `--user-server-port` | Override user server port from env file |
| `--command-port` | Override command port from env file |

### Running

It relies on [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) for SQLite support, which requires CGO:

```
cd tiauth
CGO_ENABLED=1 go run . --env-file .env.test
```

### Building

```
cd tiauth
CGO_ENABLED=1 go build .
```

### Releasing a new version

The Go module lives in the `tiauth/` subdirectory, so git tags must be prefixed with `tiauth/`:

```
git tag -a tiauth/v0.2.0 -m "description of changes"
git push origin tiauth/v0.2.0
```

The Go module proxy (`proxy.golang.org`) picks up the tag automatically. No CI or publish step is needed.

To update a downstream project (e.g. dodeka):

```
go get github.com/tiptenbrink/tiauth-faroe/tiauth@v0.2.0
go mod tidy
```

`go mod tidy` removes checksums for old versions from `go.sum` that are no longer needed.

## Python packages

### Client (`python/client`)

Client library for communicating with a Tiauth Faroe server. Provides both sync and async interfaces for action invocation.

### User server (`python/user-server`)

Library to help implement a user store backend that the Tiauth Faroe server communicates with. Defines dataclasses for every operation (effects) and `AsyncServer`/`SyncServer` protocols, so you can customize the function signatures while using `handle_request_sync` or `handle_request_async` to process requests.

### Sync reference server (`python/sync-reference-server`)

A reference implementation of the sync user server interface. This is a local development/testing tool and is not published as a package.
