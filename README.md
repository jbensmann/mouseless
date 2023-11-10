# mouseless

This program allows you to control the mouse pointer in Linux with the keyboard. It works in all Linux distributions,
even those running with Wayland.

It is the successor of [xmouseless](https://github.com/jbensmann/xmouseless), which depended on X11 and had some
minor issues.

## Features

- move the pointer continuously
- change the pointer speed on the fly
- click, grab, scroll
- remap keys
- define arbitrary layers

## Why

There are various reasons why one would want to control the mouse with the keyboard:

- keep your hands on the keyboard
- laptop with no or a poor touchpad
- no mouse at hand
- cannot use a mouse for some reason
- precise control
- for fun

## Installation

The simplest way is to download a binary from [Releases](https://github.com/jbensmann/mouseless/releases).

Or you can build it from source (requires that go is installed):

```shell
go build -ldflags="-s -w" .
```

When successful, a binary with name `mouseless` will pop out.

## Usage

First you need to create a config file, e.g. `~/.config/mouseless/config.yaml`, see below for an example.

Then you can run mouseless like this:

```shell
sudo mouseless --config ~/.config/mouseless/config.yaml
```

For troubleshooting, you can use the --debug flag to show more verbose log messages.

## Configuration

The format of the configuration file is YAML, you do not have to know what exactly that is, just take care
that the indentation level of the lines is correct. Lines starting with a `#` are comments.
Here is a small example that illustrates the most features:

```yaml
# the keyboard devices it reads from, if no devices are specified, it reads from all
devices:
# - "/dev/input/by-id/SOME_KEYBOARD_REPLACE_ME-event-kbd"

# this is executed when mouseless starts, e.g. useful for setting the keyboard layout
# startCommand: "setxkbmap de"

# the default speed for mouse movement and scrolling
baseMouseSpeed: 750.0
baseScrollSpeed: 20.0

# the time it takes to accelerate to baseMouseSpeed (in ms), 0 to reach top speed immediately
mouseAccelerationTime: 200.0
# the shape of the acceleration curve, 1.0 is linear, higher values have more time at low speeds
mouseAccelerationCurve: 2.0
# speed of the mouse when it starts moving
startMouseSpeed: 0.0
# same for deceleration
mouseDecelerationTime: 300.0
mouseDecelerationCurve: 3.0

# the rest of the config defines the layers with their bindings
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
  passThrough: false
  bindings:
    e: up
    s: left
    d: down
    f: right
    q: esc
    w: backspace
    r: delete
    v: enter
    # _ is the wildcard key, which matches any key that is not mapped
    _: rightalt+_
```

One can define an arbitrary number of layers, each with an arbitrary number of bindings, e.g. `esc: capslock`
which maps the escape key to capslock. If you do not know the name of a key, you can start mouseless with the
--debug flag, press the key and look for an output like `Pressed:  rightalt (100)`, which tells you that the name of the
key is `rightalt`. Alternatively you can also use the keycode in the parentheses, which is 100 in this case. Note that
the name of a key does not necessarily match what is printed on your keyboard, e.g. with a German layout where the `y`
and `z` keys are swapped in comparison to the English layout, but the name of the `z` key is `y` and vice versa.

One can also map a key to multiple ones like `a: leftshift+k1` which results in `!`, at least for an English or German
layout.

Aside from remapping keys, there are a bunch of other actions available, e.g. `rightalt: toggle-layer arrows`, which
jumps to the arrows layer when rightalt is pressed and jumps back on release. These are all available actions:

| action                 | examples                                  | meaning                                                                   |
|------------------------|-------------------------------------------|---------------------------------------------------------------------------|
| `<key-combo>`          | `a`, `comma`, `shift+a`                   | maps to the key (combo)                                                   |
| `layer <layer>`        | `layer mouse`                             | switches to the layer with the given name                                 |
| `toggle-layer <layer>` | `toggle-layer mouse`                      | switches to the layer with the given name while the mapped key is pressed |
| `move <x> <y>`         | `move 1 0`                                | moves the pointer into the given direction                                |
| `scroll <direction>`   | `scroll up`                               | scrolls up or down                                                        |
| `speed <multiplier>`   | `speed 2.5`                               | multiplies the pointer and scroll speeds with the given value             |
| `button <button>`      | `button left`                             | presses a mouse button (left, right or middle)                            |
| `exec <cmd>`           | `exec notify-send "hello from mouseless"` | executes the given command (the example sends a desktop notification)     |
| `reload-config`        | `reload-config`                           | reloads the configuration file                                            |

With these actions one could e.g. toggle the mouse layer with `tab: toggle-layer mouse`, so that all bindings from the
mouse layer are available while `tab` is held down. However, this sacrifices the `tab` key which might not be desirable.
For these cases there are some "meta actions" which allow to put multiple actions on a single key and which are inspired
by KMonad. The arguments of those actions have to be separated with `;`.

| meta action                                                    | example                                            | meaning                                                                                                                       |
|----------------------------------------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------|
| `tap-hold <tap action>; <hold action>; <timeout>`              | `tap-hold a; toggle-layer mouse; 300`              | when the mapped key is pressed and released within 300ms, presses a, otherwise toggles the mouse layer                        |
| `tap-hold-next <tap action>; <hold action>; <timeout>`         | `tap-hold-next a; toggle-layer mouse; 300`         | same as tap-hold, with the addition that the tap action is executed when another key is pressed while `a` is still held down  |
| `tap-hold-next-release <tap action>; <hold action>; <timeout>` | `tap-hold-next-release a; toggle-layer mouse; 300` | same as tap-hold, with the addition that the tap action is executed when another key is released while `a` is still held down |
| `multi <action1>; <action2>`                                   | `multi a; toggle-layer mouse`                      | executes two or more actions at once                                                                                          |

## Custom devices

If you don't want mouseless to read from all keyboards, you can specify one or more devices in the configuration file.
Most devices have `kbd` in their name, so you can use the following commands to find possible candidates:

```shell
ls /dev/input/by-id/*kbd*
ls /dev/input/by-path/*kbd*
```

## Run without root privileges

To run without using sudo, you can add an udev rule with the following command, which allows your user to read from
keyboard devices and create a virtual keyboard and mouse:

```sh
sudo tee /etc/udev/rules.d/99-$USER.rules <<EOF
KERNEL=="uinput", GROUP="$USER", MODE:="0660"
KERNEL=="event*", GROUP="$USER", NAME="input/%k", MODE="660"
EOF
```

To apply the changes, you can simply reboot your machine.
In case that does not work, it might be that the uinput kernel module is not loaded, which can be fixed with this,
followed by a reboot:

```sh
echo "uinput" | sudo tee /etc/modules-load.d/uinput.conf
```

## Run at startup without typing login passcode

For most distros, we can run mouseless at startup using `systemd`.

### With root privileges

1. Download and move the latest release of mouseless to a directory, e.g. `/usr/local/bin/` in elevated previliges.
   Make sure to complete the **entire process** including download in elevated previliges (i.e. `sudo su`),
   otherwise the program might not be able to be executed using systemd.
2. Make a file called `mouseless.service` in `/etc/systemd/system/` with the following countent.
   Note if you may need to set the `mouseless` command with absolute path for the `ExecStart`.
   ```
   [Unit]
   Description=Mouseless Service

   [Service]
   Type=simple
   ExecStart=/path/to/mouseless --config /path/to/config.yaml
   Restart=on-failure
   RestartSec=10
   KillMode=process

   [Install]
   WantedBy=multi-user.target
   ```
3. Enable and start the mouseless service using the following command:
   ```sh
   sudo systemctl enable mouseless.service
   sudo systemctl start mouseless.service
   ```
   and you can check the status of mouseless via the following command:
   ```sh
   sudo systemctl status mouseless.service
   ```

### Without root privileges

1. Download and move the latest release of mouseless to a `bin` directory, e.g. `$HOME/.local/bin/`.
2. Make the file `mouseless.service` mentioned in the previous section in `$HOME/.config/systemd/user/`.
3. Enable and start the mouseless service using the following command:
   ```sh
   systemctl --user enable mouseless.service
   systemctl --user start mouseless.service
   ```
   and you can check the status of mouseless via the following command:
   ```sh
   systemctl --user status mouseless.service
   ```
