#!/bin/bash
set -e

# spank-claude installer
# One script to set up Claude Code approval via laptop slap.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/usr/local/bin"
SPANK_BIN="$INSTALL_DIR/spank"
CLAUDE_SETTINGS="$HOME/.claude/settings.json"
SUDOERS_FILE="/etc/sudoers.d/spank"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENSURE_SCRIPT="$SCRIPT_DIR/ensure-running.sh"
SOUND="sexy"

echo -e "${GREEN}spank-claude installer${NC}"
echo "Approve Claude Code actions by slapping your laptop."
echo ""

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        --sound) SOUND="$2"; shift 2;;
        *) echo "Unknown option: $1"; exit 1;;
    esac
done

if [[ "$SOUND" != "pain" && "$SOUND" != "sexy" && "$SOUND" != "halo" ]]; then
    echo -e "${RED}Invalid sound pack: $SOUND (choose: pain, sexy, halo)${NC}"
    exit 1
fi

# 1. Check requirements
echo "Checking requirements..."

if [[ "$(uname -m)" != "arm64" ]]; then
    echo -e "${RED}Apple Silicon (M-series) required.${NC}"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Go not found. Installing via Homebrew...${NC}"
    brew install go
fi

# 2. Build
echo "Building spank..."
cd "$SCRIPT_DIR"
go build -o spank .
echo -e "${GREEN}Built successfully.${NC}"

# 3. Install binary (needs sudo)
echo ""
echo "Installing to $SPANK_BIN (requires sudo)..."
sudo cp "$SCRIPT_DIR/spank" "$SPANK_BIN"
sudo chmod +x "$SPANK_BIN"
echo -e "${GREEN}Binary installed.${NC}"

# 4. Set up sudoers NOPASSWD
echo "Setting up passwordless sudo for spank..."
echo "$USER ALL=(root) NOPASSWD: $SPANK_BIN" | sudo tee "$SUDOERS_FILE" > /dev/null
sudo chmod 0440 "$SUDOERS_FILE"
if sudo visudo -cf "$SUDOERS_FILE" > /dev/null 2>&1; then
    echo -e "${GREEN}Sudoers rule installed.${NC}"
else
    echo -e "${RED}Sudoers validation failed! Removing...${NC}"
    sudo rm -f "$SUDOERS_FILE"
    exit 1
fi

# 5. Update ensure-running.sh with chosen sound
sed -i '' "s/--sound [a-z]*/--sound $SOUND/" "$ENSURE_SCRIPT"
chmod +x "$ENSURE_SCRIPT"

# 6. Configure Claude Code hooks
echo "Configuring Claude Code hooks..."
mkdir -p "$HOME/.claude"

if [ -f "$CLAUDE_SETTINGS" ]; then
    # Check if hooks already configured
    if grep -q "19222/hook" "$CLAUDE_SETTINGS" 2>/dev/null; then
        echo -e "${YELLOW}Claude Code hooks already configured, skipping.${NC}"
    else
        # Backup and merge hooks into existing settings
        cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup"
        # Use python3 (ships with macOS) to merge JSON
        python3 -c "
import json, sys
with open('$CLAUDE_SETTINGS') as f:
    settings = json.load(f)
hooks = settings.get('hooks', {})
hooks['SessionStart'] = [{'hooks': [{'type': 'command', 'command': '$ENSURE_SCRIPT'}]}]
hooks['PermissionRequest'] = [{'hooks': [{'type': 'http', 'url': 'http://127.0.0.1:19222/hook', 'timeout': 35}]}]
settings['hooks'] = hooks
with open('$CLAUDE_SETTINGS', 'w') as f:
    json.dump(settings, f, indent=2)
    f.write('\n')
"
        echo -e "${GREEN}Hooks added to existing settings (backup: settings.json.backup).${NC}"
    fi
else
    cat > "$CLAUDE_SETTINGS" << 'SETTINGS'
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "ENSURE_PLACEHOLDER"
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
SETTINGS
    sed -i '' "s|ENSURE_PLACEHOLDER|$ENSURE_SCRIPT|" "$CLAUDE_SETTINGS"
    echo -e "${GREEN}Claude Code settings created.${NC}"
fi

# 7. Kill any existing spank and start fresh
echo "Starting spank..."
sudo pkill -f "$SPANK_BIN" 2>/dev/null || true
sleep 1
sudo "$SPANK_BIN" --sound "$SOUND" > /tmp/spank-claude.log 2>&1 &
sleep 1

if curl -s --connect-timeout 2 -X POST http://127.0.0.1:19222/hook -d '{}' > /dev/null 2>&1; then
    echo -e "${GREEN}spank is running on port 19222 (sound=$SOUND).${NC}"
else
    echo -e "${YELLOW}spank started but port check failed. Check /tmp/spank-claude.log${NC}"
fi

echo ""
echo -e "${GREEN}Done! Setup complete.${NC}"
echo ""
echo "How it works:"
echo "  1. Start Claude Code — spank launches automatically"
echo "  2. Claude requests permission — you hear 'slap me'"
echo "  3. Slap the laptop — action approved!"
echo ""
echo "Logs: /tmp/spank-claude.log"
echo "Sound: $SOUND (change with: install.sh --sound pain|sexy|halo)"
