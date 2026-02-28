# spank-claude

Approve Claude Code tool calls by physically slapping your MacBook.

Fork of [taigrr/spank](https://github.com/taigrr/spank) — the accelerometer-based slap detector, repurposed as an HTTP approval server for [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks).

## How it works

```
Claude Code requests permission (e.g. run a bash command)
        ↓
POST → spank HTTP server (localhost:19222)
        ↓
🔔 macOS Ping notification sound
        ↓
👋 You slap the laptop
        ↓
🔊 Approval sound from selected pack (pain/sexy/halo)
✅ Returns {"decision": {"behavior": "allow"}}
        ↓
Claude Code continues
```

- No slap within 30 seconds → empty response → Claude Code shows the normal terminal prompt
- Slaps without a pending request are silently ignored

## Requirements

- macOS on Apple Silicon (M2+)
- [Go](https://go.dev/) (for building; installer will install it via Homebrew if missing)

## Quick install

```bash
git clone https://github.com/K00pash/spank.git
cd spank
./install.sh
```

The script handles everything:
- Builds the binary and copies it to `/usr/local/bin/`
- Sets up passwordless `sudo` for spank (IOKit HID needs root)
- Configures Claude Code hooks in `~/.claude/settings.json`
- Starts spank in the background

Choose a sound pack:

```bash
./install.sh --sound pain   # "Ow!", "Ouch!", "Hey that hurts!"
./install.sh --sound sexy   # moans and groans (default)
./install.sh --sound halo   # Halo death sounds
```

After install, just start Claude Code — spank launches automatically via SessionStart hook.

## Sound packs

| Pack | Clips | Description |
|------|-------|-------------|
| **pain** | 10 | "Ow!", "Ouch!", "Hey that hurts!", "Yowch!" |
| **sexy** | 60 | Random moans and groans |
| **halo** | 9 | Halo game death sounds |

## Manual usage

```bash
# Start the server (requires sudo for accelerometer access)
sudo spank --sound sexy

# Custom port
sudo spank --sound pain --port 8080
```

Test with curl (in another terminal):

```bash
curl -X POST http://127.0.0.1:19222/hook \
  -d '{"hook_event_name":"PermissionRequest","tool_name":"Bash"}'
```

You should hear a Ping → slap the laptop → curl receives the approval JSON.

## What the installer configures

Two hooks in `~/.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/spank/ensure-running.sh"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19222/hook",
            "timeout": 35
          }
        ]
      }
    ]
  }
}
```

- **SessionStart** — ensures spank is running when you launch Claude Code
- **PermissionRequest** — routes every tool permission request through spank

## Uninstall

```bash
sudo rm /usr/local/bin/spank
sudo rm /etc/sudoers.d/spank
sudo pkill -f /usr/local/bin/spank
# Remove hooks from ~/.claude/settings.json manually
```

## Differences from the original spank

| Original | This fork |
|----------|-----------|
| Plays sounds on every slap | Sounds **only** when approving Claude Code actions |
| Standalone CLI toy | HTTP server for Claude Code hooks |
| `--sexy` / `--halo` flags | `--sound pain\|sexy\|halo` flag |
| No network | `POST /hook` endpoint on localhost |
| Always listening | Ignores slaps when no permission request is pending |

Reused from the original: accelerometer detection via IOKit HID, sound packs, beep audio playback.

## Credits

- Original [spank](https://github.com/taigrr/spank) by [@taigrr](https://github.com/taigrr)
- Sensor and detection from [apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer)

## License

MIT
