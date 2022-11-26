#!/bin/sh
version=0.1.3
# if the 1st command fails, the second will activate.
keyboard=$(ls /dev/input/by-id/*kbd* || ls /dev/input/by-path/*kbd* | head -n1)
temporary=/tmp/mouseless
# nv means no verbose
wget -P $temporary https://github.com/jbensmann/mouseless/releases/download/v$version/mouseless-linux-amd64.tar.gz
tar -xf $temporary/mouseless-linux-amd64.tar.gz --directory $temporary
sudo cp $temporary/dist/mouseless /usr/local/bin/
# copies the default config to the .config directory
mkdir /home/$USER/.config/mouseless
touch /home/$USER/.config/mouseless/config.yaml 
echo "
devices:
# change this to a keyboard device
- "$keyboard"
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
    v: enter" > /home/$USER/.config/mouseless/config.yaml 
# Creates an entry that automatically launches mouseless with the --config flag
touch $temporary/mouseless-execute
echo "
#!/bin/sh 
sudo mouseless --config ~/.config/mouseless/config.yaml" > $temporary/mouseless-execute
chmod +x $temporary/mouseless-execute
sudo cp $temporary/mouseless-execute /usr/local/bin/
# Removes the downloaded files
echo "Installation complete. Type mouseless-execute to launch the app alongside the config file. If you want to use other flags, use mouseless. If you wish do delete these entries, type sudo rm /usr/local/bin/mouseless /usr/local/bin/mouseless-execute."
