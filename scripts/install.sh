#!/bin/bash

# homectl installer script
# This script builds and installs homectl as a systemd service.

set -e

APP_NAME="homectl"
BINARY_DEST="/usr/local/bin/${APP_NAME}"
SHARE_DEST="/usr/local/share/${APP_NAME}"
UI_DEST="${SHARE_DEST}/ui"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

echo "Building ${APP_NAME}..."

# 1. Build Go binary
mkdir -p bin
go build -o "bin/${APP_NAME}" .

# 2. Build UI
echo "Building UI..."
pushd ui
npm install
npm run build
popd

# 3. Create destination directories
echo "Creating directories..."
sudo mkdir -p "${SHARE_DEST}"
sudo mkdir -p "${UI_DEST}"

# 4. Install files
echo "Installing files..."
sudo cp "bin/${APP_NAME}" "${BINARY_DEST}"
sudo cp -r ui/dist/* "${UI_DEST}/"

# 5. Create systemd unit file
echo "Creating systemd service..."
USER_NAME=$(whoami)
cat <<EOF | sudo tee "${SERVICE_FILE}" > /dev/null
[Unit]
Description=homectl Smart Home API Server
After=network.target

[Service]
ExecStart=${BINARY_DEST} serve --port 8086 --ui ${UI_DEST}
Restart=always
User=${USER_NAME}
Environment=XDG_CONFIG_HOME=/home/${USER_NAME}/.config

[Install]
WantedBy=multi-user.target
EOF

# 6. Reload and enable
echo "Reloading systemd, enabling and restarting service..."
sudo systemctl daemon-reload
sudo systemctl enable "${APP_NAME}"
sudo systemctl restart "${APP_NAME}"

echo "Installation/Update complete!"
echo "You can start the service with: sudo systemctl start ${APP_NAME}"
echo "Logs can be viewed with: journalctl -u ${APP_NAME} -f"
