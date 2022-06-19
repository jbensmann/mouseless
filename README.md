# mouseless

This program is a replacement for the physical mouse in Linux.

It is the successor of [xmouseless](https://github.com/jbensmann/xmouseless).

## Features

- move the pointer continuously
- change the pointer speed on the fly
- click, grab, scroll
- remap keys

## Why

There are various reasons why one would want to control the mouse with the keyboard instead of a regular mouse:

- you do not want to leave the keyboard
- laptop with no or a poor touchpad
- no mouse at hand
- cannot use a mouse for some reason
- for fun

Of course, it would be best to avoid using the pointer at all, e.g. with the use of tiling window managers and
application shortcuts, but sometimes there is no way around.

## Installation

There will soon be pre-built binaries, until then you can build it from source (requires that go is installed):

```shell
go build -ldflags="-s -w" .
```

When successful, a binary with name `mouseless` will pop out.

## Usage

First you need to create a config file, e.g. `~/.config/mouseless/config.yaml`, see below for an example. In there, you
have to specify the keyboard device that mouseless should read from, you can e.g. use these commands to find possible
candidates:

```shell
ls /dev/input/by-id/*kbd*
ls /dev/input/by-path/*kbd*
```

After that, you can run mouseless like this.

```shell
sudo mouseless --config ~/.config/mouseless/config.yaml
```

For troubleshooting, you can use the --debug flag to show more verbose log messages.

## Configuration

Here is a small example that illustrates the most features:

```yaml
devices:
# change this to a keyboard device
- "/dev/input/by-id/SOME_KEYBOARD_REPLACE_ME-event-kbd"
# this is executed when mouseless starts
startCommand: "setxkbmap en"
# the default speed for mouse movement
baseMouseSpeed: 750.0
# the default speed for scrolling
baseScrollSpeed: 20.0
layers:
# the first layer is active at start
- name: initial
  bindings:
    # when tab is holt and another key pressed, activate mouse layer
    tab: tap-hold-next tab ; toggle-layer mouse ; 500
    # when a is holt for 300ms, activate mouse layer
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
```

## Run without root privileges

To run without using sudo, you can add an udev rule with the following command, which allows your user to read from
keyboard devices and create a virtual keyboard and mouse:

```sh
echo "KERNEL==\"uinput\", GROUP=\"$USER\", MODE:=\"0660\"
KERNEL==\"event*\", GROUP=\"$USER\", NAME=\"input/%k\", MODE=\"660\"" \
| sudo tee /etc/udev/rules.d/99-$USER.rules
```

To apply the changes, you can simply reboot your machine.

