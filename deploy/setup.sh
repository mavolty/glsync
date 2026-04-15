#!/usr/bin/env bash
# glsync production setup for Ubuntu/Debian
# Run as root: sudo bash deploy/setup.sh
set -euo pipefail

BINARY_NAME="glsync"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/glsync"
SYSTEMD_DIR="/etc/systemd/system"

echo "=== glsync Production Setup ==="

# 1. Install PostgreSQL
echo ""
echo "[1/5] Installing PostgreSQL..."
if ! command -v psql &>/dev/null; then
    apt update && apt install -y postgresql postgresql-contrib
    systemctl enable postgresql
    systemctl start postgresql
    echo "PostgreSQL installed."
else
    echo "PostgreSQL already installed."
fi

# 2. Create database and user
echo ""
echo "[2/5] Creating database..."
sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='glsync'" | grep -q 1 || \
    sudo -u postgres createuser --createdb glsync
sudo -u postgres psql -lqt | cut -d\| -f1 | grep -qw glsync || \
    sudo -u postgres createdb -O glsync glsync
echo "Database 'glsync' ready."

# 3. Create system user and config directory
echo ""
echo "[3/5] Setting up config..."
id -u glsync &>/dev/null || useradd --system --no-create-home --shell /usr/sbin/nologin glsync
mkdir -p "$CONFIG_DIR"
cp config/glsync.yaml "$CONFIG_DIR/glsync.yaml"
cp .env.example "$CONFIG_DIR/env"
chmod 600 "$CONFIG_DIR/env"
chown -R glsync:glsync "$CONFIG_DIR"
echo "Config dir: $CONFIG_DIR"
echo ""
echo ">>> EDIT $CONFIG_DIR/env and set all required variables <<<"
echo "    Required: GLSYNC_GITLAB__WEBHOOK_SECRET, GLSYNC_JIRA__USERNAME,"
echo "    GLSYNC_JIRA__API_TOKEN, GLSYNC_DATABASE__URL"
echo "    Optional: GLSYNC_SERVER__ADMIN_TOKEN"
echo ""

# 4. Build and install binary
echo "[4/5] Building binary..."
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$INSTALL_DIR/$BINARY_NAME" ./cmd/glsync
echo "Installed: $INSTALL_DIR/$BINARY_NAME"

# 5. Install systemd service
echo ""
echo "[5/5] Installing systemd service..."
cp deploy/glsync.service "$SYSTEMD_DIR/$BINARY_NAME.service"
systemctl daemon-reload
systemctl enable "$BINARY_NAME"
echo "Service installed and enabled."

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. Edit /etc/glsync/env — fill in all secrets"
echo "  2. Run migrations:"
echo "     migrate -path migrations -database 'postgres://glsync:@localhost:5432/glsync' up"
echo "     (Set the password in the URL — check /etc/glsync/env for DATABASE_URL)"
echo "  3. Discover Jira 'done' transition ID:"
echo "     curl -u EMAIL:TOKEN https://JIRA_BASE/rest/api/2/issue/RFQA-TICKET/transitions"
echo "     Then set done transition in /etc/glsync/glsync.yaml"
echo "  4. Start the service:"
echo "     systemctl start glsync"
echo "  5. Check status:"
echo "     systemctl status glsync"
echo "     journalctl -u glsync -f"
