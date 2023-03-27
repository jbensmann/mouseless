#!/usr/bin/env sh
version=0.1.4
temporary=/tmp/mouseless

echo "=========================================="
echo "Install mouseless version $version? [y/n]"
echo "=========================================="
read choice
if [ "$choice" == "y" ]; then 
    echo "Installing."
    wget -q -P $temporary https://github.com/jbensmann/mouseless/releases/download/v$version/mouseless-linux-amd64.tar.gz && \
    tar -xf $temporary/mouseless-linux-amd64.tar.gz --directory $temporary && \
    sudo mv $temporary/dist/mouseless /usr/local/bin/ && \
    echo "Installed to /usr/local/bin/mouseless" || echo "Failed to install mouseless."
    # helps the user see what's going on
    sleep 2
else
echo "Skipping."
fi

echo ""
echo "=========================================="
echo "Finding keyboard devices."
echo "=========================================="
sleep 1
keyboards=$(find /dev/input/by-id /dev/input/by-path -name "*kbd*")
if [ -z "$keyboard" ]; then
    echo "$keyboards"
    # simply pick the first one
    keyboard="$(echo "$keyboards" | head -n1)"
else
    keyboard="replace_me"
    echo "No keyboard device found."
fi
sleep 2

# create a config file if it does not exist
if [ ! -f ~/.config/mouseless/config.yaml ]; then
    echo ""
    echo "=========================================="
    echo "Creating a config file."
    echo "=========================================="
    echo "Using device: $keyboard"

    mkdir -p ~/.config/mouseless
    echo '
devices:
# change this to a keyboard device
- "'$keyboard'"
# this is executed when mouseless starts
# startCommand: ""
# the default speed for mouse movement
baseMouseSpeed: 750.0
# the default speed for scrolling
baseScrollSpeed: 20.0
layers:
# the first layer is active at start
- name: initial
  bindings:
    # when tab is held and another key pressed, activate mouse layer
    tab: tap-hold-next tab ; toggle-layer mouse ; 500
    # when a is held for 300ms, activate mouse layer
    a: tap-hold a ; toggle-layer mouse ; 300
    # right alt key toggles arrows layer
    rightalt: toggle-layer arrows
    # switch escape with capslock
    esc: capslock
    capslock: esc
# a layer for mouse movement
- name: mouse
  # when true, keys that are not mapped keep their original meaning
  passThrough: true
  bindings:
    # quit mouse layer
    q: layer initial
    # keep the mouse layer active
    space: layer mouse
    r: reload-config
    l: move  1  0
    j: move -1  0
    k: move  0  1
    i: move  0 -1
    p: scroll up
    n: scroll down
    leftalt: speed 4.0
    e: speed 0.3
    capslock: speed 0.1
    f: button left
    d: button middle
    s: button right
    # move to the top left corner
    k0: "exec xdotool mousemove 0 0"
# another layer for arrows and some other keys
- name: arrows
  passThrough: true
  bindings:
    e: up
    s: left
    d: down
    f: right
    q: esc
    w: backspace
    r: delete
    v: enter
' > ~/.config/mouseless/config.yaml 
    echo "Created ~/.config/mouseless/config.yaml"
else
    echo ""
    echo "Config file will NOT be created because it already exists."
fi

sleep 2
echo ""
echo "=========================================="
echo "Installation complete."
echo "=========================================="
echo "You can run it with:"
echo "sudo mouseless --debug --config ~/.config/mouseless/config.yaml"
