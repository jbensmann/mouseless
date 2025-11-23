#!/usr/bin/env sh
echo "Downloading the latest version of mouseless..."
TMPDIR="$(mktemp -d)"
cd "$TMPDIR" || exit 1
curl -L --progress-bar -o mouseless-linux-amd64.tar.gz \
  "https://github.com/jbensmann/mouseless/releases/latest/download/mouseless-linux-amd64.tar.gz" || exit 1
tar -xf mouseless-linux-amd64.tar.gz || exit 1
echo "Installing to /usr/local/bin/mouseless"
sudo install -m 755 dist/mouseless /usr/local/bin/mouseless || exit 1

# create a config file if it does not exist
CONFIGDIR="$HOME/.config/mouseless"
CONFIG="$CONFIGDIR/config.yaml"
if [ ! -f "$CONFIG" ]; then
    echo "Creating config file $CONFIG"
    mkdir -p "$CONFIGDIR"
    curl -L --progress-bar -o "$CONFIG" https://raw.githubusercontent.com/jbensmann/mouseless/refs/heads/main/example_configs/config_minimal.yaml
else
    echo "Config file will NOT be created because it already exists."
fi

echo "Installation complete."
echo "You can run mouseless with:"
echo "sudo mouseless --config ~/.config/mouseless/config.yaml"
