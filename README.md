# BetterScreen

A terminal UI for managing GNU Screen sessions, in the style of lazygit and
lazydocker. List, attach, create and kill sessions, browse their windows, and
jump between them — including from inside a running session.

## Features

- List all sessions (attached / detached / dead) and attach to one — a session
  still attached elsewhere is detached from that terminal and reattached here.
- Detach a session from its terminal without killing it.
- Create and kill sessions from the interface.
- Browse the windows of a session and attach directly to a chosen window.
- Show each window's working directory and foreground process (best effort).
- In-session mode: open the menu from inside a session to switch windows
  without detaching, or jump to another session in one step.

## Requirements

- Go 1.24 or newer (the module pins `go 1.24`; the toolchain is fetched
  automatically when `GOTOOLCHAIN=auto`).
- GNU Screen 4.09 or newer, available on the `PATH`.

## Installation

Build the binary into a directory on the `PATH`:

```
go build -o ~/.local/bin/betterscreen .
```

## Usage

Run the launcher from a terminal:

```
betterscreen
```

Navigate the session and window panels, then press Enter to attach. Detaching
from a session (`Ctrl-A d`) returns to the menu.

## In-session mode

BetterScreen also runs from inside a screen session. For seamless
session-to-session jumps, use BetterScreen as the terminal launcher (it
replaces the common `screen -r "$(... fzf)"` alias): attach to sessions from it.

Add the following to `~/.screenrc` to open the menu with `Ctrl-A g`:

```
bind g screen -t betterscreen betterscreen
```

The `betterscreen` binary must be on the `PATH`. The `g` key overrides screen's
default `vbell` toggle; pick another key if needed.

Inside the in-session menu:

- the current session is marked with `●`;
- selecting a window of the current session switches to it without detaching;
- selecting another session jumps to it (the current session is detached and
  the launcher attaches the target automatically).

## API service

`betterscreen serve` exposes the sessions over HTTPS so a separate front-end
can list and manage them and drive their terminals. BetterScreen ships the API
only, no web UI.

```
betterscreen serve [--addr :8443] [--config-dir DIR] [--allow-origin URL]...
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--addr` | `:8443` | Listen address. |
| `--config-dir` | `$XDG_CONFIG_HOME/betterscreen`, else `~/.config/betterscreen` | Token and TLS material. |
| `--allow-origin` | none | Origin allowed by CORS and the WebSocket check (repeatable), e.g. `https://front.lan`. |

On first start the service generates, in the config directory:

- `token` (mode 0600) — printed once. Every request needs it.
- `cert.pem` / `key.pem` — a self-signed certificate. Its SHA-256 fingerprint
  is printed at every start; check it when your browser asks to trust it.
  Delete both files to regenerate them (e.g. after the machine's IP changes).

To run it as a user service:

```
cp contrib/betterscreen.service ~/.config/systemd/user/
systemctl --user enable --now betterscreen
journalctl --user -u betterscreen   # first start prints the token here
```

### Endpoints

All requests carry `Authorization: Bearer <token>`. Errors are
`{"error": "..."}`.

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| GET | `/api/sessions` | — | `200` list of sessions |
| POST | `/api/sessions` | `{"name": "work"}` | `201` the created session (`409` if the name is taken) |
| DELETE | `/api/sessions/{id}` | — | `204` |
| GET | `/api/sessions/{id}/windows` | — | `200` windows with `pwd` and `proc` |
| POST | `/api/sessions/{id}/detach` | — | `204` |
| POST | `/api/sessions/{id}/select` | `{"window": 2}` | `204` |
| GET | `/api/events` | — | Server-Sent Events: `sessions` with the full list on connect and on every change |
| GET | `/ws/sessions/{id}?mode=share\|take&window=N` | — | WebSocket terminal stream |

A session is `{"id": "3378954.work", "pid": 3378954, "name": "work", "state": "attached"}`.

The WebSocket carries raw terminal bytes in binary frames both ways; the client
sends `{"type": "resize", "cols": 120, "rows": 40}` as a text frame on resize.
`share` (default) joins the session next to its other displays; `take`
detaches them first, so the window follows the client's size. When the screen
client exits the socket closes with code `4000`.

Browsers cannot set headers on a WebSocket: offer the subprotocols
`["betterscreen", "token.<token>"]` instead. The token is never accepted in the
URL.

### Front-end constraints

- Read `/api/events` with `fetch()` and a streamed body (for example
  `@microsoft/fetch-event-source`): the native `EventSource` cannot send the
  `Authorization` header.
- Keep mouse reporting off in the terminal emulator (xterm.js). GNU Screen
  corrupts SGR mouse reports, so mouse movements would be typed into the
  session as text.

## Keybindings

| Key | Action |
| --- | --- |
| `↑` `↓` / `j` `k` | Move selection |
| `Tab` | Switch panel |
| `Enter` | Attach (launcher) / switch or jump (in-session) |
| `x` | Detach session (attached sessions only) |
| `n` | New session |
| `d` | Kill session (with confirmation) |
| `r` | Refresh |
| `q` | Quit |

## License

[MIT](LICENSE)
