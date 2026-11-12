# BetterScreen API

`betterscreen serve` exposes the local GNU Screen sessions over HTTPS: REST to
list and manage them, Server-Sent Events to follow changes, and a WebSocket to
drive a session's terminal. It serves the API only, no web UI.

- [Running the service](#running-the-service)
- [Configuration](#configuration)
- [Authentication](#authentication)
- [CORS and origins](#cors-and-origins)
- [Conventions](#conventions)
- [Data types](#data-types)
- [REST routes](#rest-routes)
- [Events (SSE)](#events-sse)
- [Terminal (WebSocket)](#terminal-websocket)
- [Limits and timeouts](#limits-and-timeouts)
- [Front-end notes](#front-end-notes)

## Running the service

```
betterscreen serve [--addr ADDR] [--config-dir DIR] [--allow-origin URL]...
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--addr` | `addr` from `config.json`, else `:8443` | Listen address. |
| `--config-dir` | `~/.betterscreen` | Directory holding `config.json` and the TLS material. |
| `--allow-origin` | none | Origin allowed by CORS and the WebSocket check, added to `allowed_origins`. Repeatable. |

At start the service prints:

```
token: <token> (saved to /home/me/.betterscreen/config.json)   # first start only
TLS certificate SHA-256: AB:CD:...                              # every start
listening on https://[::]:8443
```

then one line per request: `METHOD /path STATUS duration`, plus
`ws open <id> <mode>` / `ws close <id> <mode> <duration>` for terminals. The
query string and headers are never logged.

`SIGINT` / `SIGTERM` stop it cleanly: open SSE and WebSocket streams are closed
and the process exits with code 0.

As a systemd user service:

```
cp contrib/betterscreen.service ~/.config/systemd/user/
systemctl --user enable --now betterscreen
loginctl enable-linger      # optional: run without an open login session
```

## Configuration

Everything lives in the config directory (`~/.betterscreen` by default):

| File | Mode | Content |
| --- | --- | --- |
| `config.json` | `0600` | Token and settings, see below. |
| `cert.pem` | `0644` | Self-signed TLS certificate. |
| `key.pem` | `0600` | Its private key. |

The directory is created with mode `0700`.

### `config.json`

```json
{
  "token": "Zk3x...43 characters...",
  "addr": ":8443",
  "allowed_origins": ["https://front.lan", "http://localhost:5173"]
}
```

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `token` | string | no | Bearer token, at least 32 characters. Generated when missing. |
| `addr` | string | no | Listen address (`host:port` or `:port`). Default `:8443`. |
| `allowed_origins` | string[] | no | Origins allowed by CORS and the WebSocket check, each `scheme://host[:port]` with `http` or `https`, no path. |

Rules:

- When the file is missing, or has no `token`, the service generates a token
  (32 random bytes, base64url, 43 characters), writes the file back with mode
  `0600`, and prints the token once. Other keys already in the file are kept.
- Unknown keys are an error, so a typo (`"adress"`) fails at start instead of
  being ignored. A token shorter than 32 characters and a malformed origin are
  errors too.
- Precedence: command-line flag, then `config.json`, then the default. Origins
  from `--allow-origin` are added to `allowed_origins`.
- The file is read at start only: restart the service after editing it.
- To rotate the token, delete the `token` key (or set a new value of at least
  32 characters) and restart.

### TLS certificate

`cert.pem` and `key.pem` are generated on first start: ECDSA P-256, valid one
year, with SANs for `localhost`, the hostname and every non-loopback IP of the
machine. The SHA-256 fingerprint is printed at every start; compare it with the
one the browser shows before trusting the certificate. Delete both files and
restart to regenerate them, for example after the machine's IP changes or the
certificate expires.

## Authentication

Every route requires the token, except CORS preflights (`OPTIONS`):

- HTTP: `Authorization: Bearer <token>`.
- WebSocket from a browser, which cannot set headers: offer the subprotocols
  `betterscreen` and `token.<token>`. The server answers with `betterscreen`.

The token is never accepted in the URL. A missing or wrong token gives:

```
401 Unauthorized
{"error":"unauthorized"}
```

## CORS and origins

For an `Origin` listed in `allowed_origins`, responses carry
`Access-Control-Allow-Origin: <origin>` and `Vary: Origin`. Preflights
(`OPTIONS` on any path) are answered with `204` before authentication and, for
an allowed origin, add:

```
Access-Control-Allow-Headers: Authorization, Content-Type
Access-Control-Allow-Methods: GET, POST, DELETE
```

The WebSocket upgrade rejects (`403`) a cross-origin request whose host is not
in `allowed_origins`. Same-origin requests and clients that send no `Origin`
(curl, native apps) are accepted.

## Conventions

- Base URL: `https://<host>:8443`.
- Request and response bodies are JSON (`Content-Type: application/json`).
  Request bodies are limited to 4 KiB.
- Errors are `{"error": "<message>"}` with the HTTP status:

| Status | When |
| --- | --- |
| `400` | Invalid JSON body, invalid name, window or mode. |
| `401` | Missing or wrong token. |
| `404` | Unknown session, malformed id, or unknown route. |
| `405` | Known route, wrong method. |
| `409` | Session name already in use. |
| `500` | `screen` failed; the message is screen's error. |

- `{id}` is a session id as returned by the API: `<pid>.<name>`, matching
  `^[0-9]+\.[A-Za-z0-9_.-]+$`. A session whose name falls outside this pattern
  is listed but answers `404` on the routes that take an id.
- Window numbers are integers in `0..99`.

## Data types

### Session

```json
{"id": "3378954.work", "pid": 3378954, "name": "work", "state": "attached"}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | string | `<pid>.<name>`, the identifier used in paths. |
| `pid` | int | PID of the screen server. |
| `name` | string | Session name. |
| `state` | string | `attached`, `detached` or `dead`. |

### Window

```json
{"num": 0, "title": "zsh", "pwd": "/home/me/project", "proc": "vim"}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `num` | int | Window number. |
| `title` | string | Window title. |
| `pwd` | string | Working directory of the window's shell, `""` if unknown. |
| `proc` | string | Foreground process name, `""` if unknown. |

`pwd` and `proc` are best effort and may be empty.

## REST routes

### `GET /api/sessions`

Lists the sessions.

```
200 OK
[{"id":"3378954.work","pid":3378954,"name":"work","state":"attached"}]
```

An empty list is `[]`.

### `POST /api/sessions`

Creates a detached session.

```json
{"name": "work"}
```

`name` must match `^[A-Za-z0-9_][A-Za-z0-9_-]{0,63}$`: letters, digits, `_`
and `-`, not starting with `-`, at most 64 characters.

| Status | Body |
| --- | --- |
| `201` | The created [Session](#session). |
| `400` | `invalid JSON body` or `invalid session name`. |
| `409` | `session name already in use`. |

### `DELETE /api/sessions/{id}`

Kills the session and all its windows. `204` on success, `404` if unknown.

### `GET /api/sessions/{id}/windows`

Lists the windows of a session, as an array of [Window](#window). `404` if the
session is unknown.

### `POST /api/sessions/{id}/detach`

Detaches every display attached to the session; the session keeps running.
`204` on success, `404` if unknown.

### `POST /api/sessions/{id}/select`

Switches the session's current window.

```json
{"window": 2}
```

| Status | Body |
| --- | --- |
| `204` | — |
| `400` | `invalid JSON body` or `window must be an integer in 0..99`. |
| `404` | `session not found`. |

### Example

```
TOKEN=$(jq -r .token ~/.betterscreen/config.json)
curl -k -H "Authorization: Bearer $TOKEN" https://localhost:8443/api/sessions
curl -k -H "Authorization: Bearer $TOKEN" -d '{"name":"work"}' https://localhost:8443/api/sessions
```

`-k` skips certificate verification; prefer `--cacert ~/.betterscreen/cert.pem`
with `https://localhost`.

## Events (SSE)

### `GET /api/events`

A `text/event-stream` that pushes the full session list on connect, then each
time it changes:

```
event: sessions
data: [{"id":"3378954.work","pid":3378954,"name":"work","state":"detached"}]

: keep-alive
```

- The server polls `screen -ls` every 2 seconds while at least one client is
  subscribed, and sends only when the list differs from the previous one.
- `data` is the same array as `GET /api/sessions`. A slow client skips
  intermediate lists and receives the latest one.
- A `: keep-alive` comment is sent every 30 seconds.
- The stream has no `id:` field; after a reconnect, the first event carries the
  full current list.

## Terminal (WebSocket)

### `GET /ws/sessions/{id}?mode=share|take&window=N`

Upgrades to a WebSocket connected to a new screen client on the session. Each
connection gets its own pty, started at 80x24 with `TERM=xterm-256color`.

| Query | Default | Meaning |
| --- | --- | --- |
| `mode` | `share` | `share`: joins the session next to its other displays (`screen -x`). `take`: detaches the other displays first (`screen -d -r`), so the window follows this client's size. |
| `window` | current window | Window to open, `0..99`. Without it the client lands on the session's current window. |

Errors before the upgrade are plain HTTP: `400` (bad `mode` or `window`), `401`,
`404` (unknown session), `403` (origin not allowed).

Frames:

| Direction | Type | Content |
| --- | --- | --- |
| server → client | binary | Raw terminal output. |
| client → server | binary | Raw keyboard input, written to the pty as is. |
| client → server | text | Control message, JSON. |

The only control message is resize, to send on connect and on every size
change:

```json
{"type": "resize", "cols": 120, "rows": 40}
```

Other text frames, and resizes with a zero dimension, are ignored.

Close codes:

| Code | Sent by | Meaning |
| --- | --- | --- |
| `1000` | server | Normal close, after the client closed its side. |
| `1009` | server | A client frame exceeded 1 MiB. |
| `1011` | server | The screen client could not start. |
| `4000` | server | The screen client exited: the session was killed, or another connection used `take`. |

When the client closes the socket, the server hangs up the pty: the screen
display detaches and the session keeps running.

Browser example:

```js
const ws = new WebSocket(
  `wss://${host}:8443/ws/sessions/${id}?mode=share`,
  ["betterscreen", `token.${token}`],
);
ws.binaryType = "arraybuffer";
ws.onopen = () => ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }));
ws.onmessage = (e) => term.write(new Uint8Array(e.data));
term.onData((d) => ws.send(new TextEncoder().encode(d)));
term.onResize(({ cols, rows }) => ws.send(JSON.stringify({ type: "resize", cols, rows })));
ws.onclose = (e) => { if (e.code === 4000) { /* session gone or taken over */ } };
```

## Limits and timeouts

| What | Value |
| --- | --- |
| Request body (REST) | 4 KiB |
| WebSocket incoming frame | 1 MiB (`1009` beyond) |
| Request headers read timeout | 10 s |
| Idle keep-alive connection | closed after 60 s |
| SSE poll interval | 2 s |
| SSE keep-alive comment | 30 s |
| Screen client exit after hang-up | 2 s, then killed |
| TLS | 1.2 minimum |

There is no WebSocket ping: a client that vanishes without closing keeps its
pty, and the session stays `attached`, until TCP gives up.

## Front-end notes

- Read `/api/events` with `fetch()` and a streamed body (for example
  `@microsoft/fetch-event-source`): the native `EventSource` cannot send the
  `Authorization` header.
- Keep mouse reporting off in the terminal emulator (xterm.js). GNU Screen
  rewrites SGR mouse reports, so mouse movements would be typed into the
  session as text.
- The browser must trust the self-signed certificate first: open
  `https://<host>:8443/api/sessions` once and accept it, after checking the
  fingerprint.
