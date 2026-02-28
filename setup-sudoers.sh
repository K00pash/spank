#!/bin/bash
# Allow running spank without password
echo "koopa ALL=(root) NOPASSWD: /usr/local/bin/spank" | sudo tee /etc/sudoers.d/spank
sudo chmod 0440 /etc/sudoers.d/spank
sudo visudo -cf /etc/sudoers.d/spank && echo "OK: sudoers rule installed" || echo "ERROR: invalid sudoers syntax"
