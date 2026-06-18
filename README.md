# BetterScreen

A terminal UI for managing GNU Screen sessions, in the style of lazygit and
lazydocker. List, attach, create and kill sessions, browse their windows, and
jump between them — including from inside a running session.

## Features

- List all sessions (attached / detached / dead) and attach to one.
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

## Keybindings

| Key | Action |
| --- | --- |
| `↑` `↓` / `j` `k` | Move selection |
| `Tab` | Switch panel |
| `Enter` | Attach (launcher) / switch or jump (in-session) |
| `n` | New session |
| `d` | Kill session (with confirmation) |
| `r` | Refresh |
| `q` | Quit |
