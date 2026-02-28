# spank-claude

Approve Claude Code actions by slapping your laptop.

Fork of [taigrr/spank](https://github.com/taigrr/spank) — converted from a standalone slap-to-sound toy into an HTTP server that integrates with [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks).

## How it works

```
Claude Code requests permission (e.g. run a bash command)
        ↓
POST → spank HTTP server (localhost:19222)
        ↓
🔔 Plays notification sound ("slap me")
        ↓
👋 You slap the laptop
        ↓
🔊 Plays approval sound (pain/sexy/halo)
✅ Returns {"decision": {"behavior": "allow"}}
        ↓
Claude Code continues
```

No slap within 30 seconds → Claude Code falls back to the normal terminal prompt.
Slaps without a pending request are silently ignored.

## Requirements

- macOS on Apple Silicon (M2+)
- [Go](https://go.dev/) (for building from source)

## Quick install

```bash
git clone https://github.com/K00pash/spank.git
cd spank
./install.sh
```

That's it. The script handles everything:
- Builds the binary and copies it to `/usr/local/bin/`
- Sets up passwordless `sudo` for spank (IOKit HID needs root)
- Configures Claude Code hooks (`~/.claude/settings.json`)
- Starts spank in the background

### Options

```bash
# Choose sound pack (default: sexy)
./install.sh --sound pain
./install.sh --sound sexy
./install.sh --sound halo
```

## Sound packs

| Pack | Description |
|------|-------------|
| **pain** | "Ow!", "Ouch!", "Hey that hurts!" — 10 clips |
| **sexy** | Escalating moans — 60 clips |
| **halo** | Halo death sounds — 9 clips |

## Manual usage

```bash
# Start the server (requires sudo for accelerometer)
sudo spank --sound sexy

# Custom port
sudo spank --sound pain --port 8080

# Test with curl
curl -X POST http://127.0.0.1:19222/hook -d '{"hook_event_name":"PermissionRequest","tool_name":"Bash"}'
```

## Claude Code configuration

The installer adds this to `~/.claude/settings.json`:

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

- **SessionStart** hook ensures spank is running when Claude Code starts
- **PermissionRequest** hook sends every permission request to spank

## Uninstall

```bash
# Remove binary
sudo rm /usr/local/bin/spank

# Remove sudoers rule
sudo rm /etc/sudoers.d/spank

# Remove hooks from ~/.claude/settings.json (edit manually)

# Stop running instance
sudo pkill -f /usr/local/bin/spank
```

## Differences from the original

| Original spank | This fork |
|----------------|-----------|
| Plays sounds on every slap | Sounds **only on Claude Code approve** |
| Standalone CLI toy | HTTP server for Claude Code hooks |
| `--sexy` / `--halo` flags | `--sound pain\|sexy\|halo` flag |
| No HTTP server | `POST /hook` endpoint on localhost |

Reused from the original: accelerometer detection, sound packs, IOKit HID sensor, beep audio playback.

## Credits

- Original [spank](https://github.com/taigrr/spank) by [@taigrr](https://github.com/taigrr)
- Sensor reading from [apple-silicon-accelerometer](https://github.com/olvvier/apple-silicon-accelerometer)

## License

MIT
