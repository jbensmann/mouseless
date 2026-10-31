# mouseless

[![lint](https://github.com/jbensmann/mouseless/actions/workflows/lint.yml/badge.svg)](https://github.com/jbensmann/mouseless/actions/workflows/lint.yml)
[![Releases](https://img.shields.io/github/downloads/jbensmann/mouseless/total)](https://github.com/jbensmann/mouseless/releases)
[![release](https://img.shields.io/github/v/release/jbensmann/mouseless)](https://github.com/jbensmann/mouseless/releases)
[![license](https://img.shields.io/github/license/jbensmann/mouseless)](https://github.com/jbensmann/mouseless/blob/main/LICENSE)

This program lets you control the mouse pointer on Linux using the keyboard. It works across all Linux distributions,
including those running Wayland.

It is the successor to [xmouseless](https://github.com/jbensmann/xmouseless).

> **Note:** There is an unrelated commercial app called *Mouseless* that unfortunately ended up with the same name. This
> project (mouseless) is free and open source.

## Features

- move the pointer continuously
- change the pointer speed on the fly
- click, grab, scroll
- remap keys
- define arbitrary layers

## Why

There are many reasons to control the mouse with the keyboard:

- keep your hands on the keyboard
- a laptop with a poor touchpad, or none at all
- no mouse nearby
- cannot use a mouse for some reason
- precise control
- for fun

## Table of Contents

- [Installation](#installation)
  - [Binary Release](#binary-release)
  - [From Source](#from-source)
- [Usage](#usage)
- [Configuration](#configuration)
- [Custom devices](#custom-devices)
- [Run without sudo](#run-without-sudo)
- [Run at startup with systemd](#run-at-startup-with-systemd)
- [Pointer acceleration](#pointer-acceleration)
- [Disable touchpad while typing](#disable-touchpad-while-typing)

## Installation

### Binary release

You can download a precompiled binary from [Releases](https://github.com/jbensmann/mouseless/releases).

The file `mouseless_linux_amd64.tar.gz` contains the executable named `mouseless`. Put it in an appropriate location and
make it executable, e.g.:

```sh
tar -xvf mouseless_linux_amd64.tar.gz
sudo mv mouseless /usr/local/bin/mouseless
sudo chmod +x /usr/local/bin/mouseless
```

Alternatively, you can use this convenience script to install the latest release and also create a config file if none
exists yet:

```sh
wget -O install.sh https://raw.githubusercontent.com/jbensmann/mouseless/refs/heads/main/install.sh
sh install.sh
```

### From source

You can download and build mouseless directly with:

```shell
go install github.com/jbensmann/mouseless@latest
```

This places the binary in `$GOPATH/bin/mouseless` (usually `~/go/bin/mouseless`).

Or you can clone and build manually:

```shell
git clone https://github.com/jbensmann/mouseless.git
cd mouseless
go build .
```

This places the binary in the current directory.

## Usage

First you need to create a config file, e.g. `~/.config/mouseless/config.yaml`, see below for an example.

Then you can run mouseless like this:

```shell
sudo mouseless --config ~/.config/mouseless/config.yaml
```

For troubleshooting, you can use the `--debug` flag to show more verbose log messages.

## Configuration

The configuration file is in YAML format. You do not need to know exactly what that means; just make sure
the indentation is correct.
Lines starting with a `#` are comments.
Here is a minimal example with only one additional layer for mouse movement:

```yaml
# the default speed for mouse movement and scrolling
baseMouseSpeed: 750.0
baseScrollSpeed: 20.0

# the rest of the config defines the layers with their bindings
layers:
# the first layer is active at start
- name: initial
  bindings:
    # when tab is held and another key pressed, activate mouse layer
    tab: tap-hold-next tab ; toggle-layer mouse ; 500
    # when a is held for 300ms, activate mouse layer
    a: tap-hold a ; toggle-layer mouse ; 300
# a layer for mouse movement
- name: mouse
  # when true, keys that are not mapped keep their original meaning
  passThrough: true
  bindings:
    # quit mouse layer
    q: layer initial
    # keep the mouse layer active
    space: layer mouse
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
```

Here you can also find a more comprehensive example that illustrates most available features and config
options: [config_full.yaml](./example_configs/config_full.yaml)

You can define an arbitrary number of layers, each with an arbitrary number of bindings, e.g. `esc: capslock`
which maps the escape key to capslock. If you do not know the name of a key, you can start mouseless with the
`--debug` flag, press the key and look for an output like `Pressed:  rightalt (100)`, which tells you that the name of the
key is `rightalt`. Alternatively you can also use the keycode in the parentheses, which is 100 in this case. Note that
the name of a key does not necessarily match what is printed on your keyboard, e.g. with a German layout where the `y`
and `z` keys are swapped in comparison to the English layout, but the name of the `z` key is `y` and vice versa.

You can also map a key to multiple ones like `a: leftshift+k1` which results in `!`, at least for an English or German
layout.

Aside from remapping keys, there are several other actions available, e.g. `rightalt: toggle-layer arrows`, which
jumps to the arrows layer when rightalt is pressed and jumps back on release. These are all available actions:

| action                              | examples                                                    | meaning                                                                                             |
|-------------------------------------|-------------------------------------------------------------|-----------------------------------------------------------------------------------------------------|
| `<key-combo>`                       | `a`, `comma`, `shift+a`                                     | maps to the key (combo)                                                                             |
| `layer <layer>`                     | `layer mouse`                                               | switches to the layer with name `mouse`                                                             |
| `toggle-layer <layer>`              | `toggle-layer mouse`                                        | switches to the layer with name `mouse` while the mapped key is pressed                             |
| `mod-layer <key> <layer>`           | `mod-layer leftctrl mouse`                                  | switches to the layer with name `mouse` for bound keys only, otherwise presses the left control key |
| `move <x> <y>`                      | `move 1 0`                                                  | moves the pointer in the given direction                                                            |
| `scroll <direction>`                | `scroll up`                                                 | scrolls up, down, left or right                                                                     |
| `speed <multiplier>`                | `speed 2.5`                                                 | multiplies the pointer and scroll speeds with the given value                                       |
| `button <button>`                   | `button left`                                               | presses a mouse button (left, right or middle)                                                      |
| `button <button> <interval>`        | `button left 200`                                           | clicks a mouse button repeatedly every 200ms while the mapped key is held down                      |
| `exec <cmd>`                        | `exec notify-send "hello from mouseless"`                   | executes the given command (the example sends a desktop notification)                               |
| `exec-press-release <cmd1>; <cmd2>` | `exec-press-release notify-send press; notify-send release` | executes different commands when the key is pressed and released                                    |
| `reload-config`                     | `reload-config`                                             | reloads the configuration file                                                                      |
| `nop`                               | `nop`                                                       | does nothing, can be used to disable a key                                                          |

With these actions you could e.g. toggle the mouse layer with `tab: toggle-layer mouse`, so that all bindings from the
mouse layer are available while `tab` is held down. However, this sacrifices the `tab` key, which might not be desirable.
For these cases there are some "meta actions" which let you put multiple actions on a single key and which are inspired
by KMonad. The arguments of those actions have to be separated with `;`.

| meta action                                                    | example                                            | meaning                                                                                                                       |
|----------------------------------------------------------------|----------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------|
| `tap-hold <tap action>; <hold action>; <timeout>`              | `tap-hold a; toggle-layer mouse; 300`              | when the mapped key is pressed and released within 300ms, presses a, otherwise toggles the mouse layer                        |
| `tap-hold-next <tap action>; <hold action>; <timeout>`         | `tap-hold-next a; toggle-layer mouse; 300`         | same as tap-hold, with the addition that the tap action is executed when another key is pressed while `a` is still held down  |
| `tap-hold-next-release <tap action>; <hold action>; <timeout>` | `tap-hold-next-release a; toggle-layer mouse; 300` | same as tap-hold, with the addition that the tap action is executed when another key is released while `a` is still held down |
| `multi <action1>; <action2>`                                   | `multi a; toggle-layer mouse`                      | executes two or more actions at once                                                                                          |
| `one-shot <action>`                                            | `one-shot leftshift`                               | executes the action while held; if tapped, it stays active for the next key press, if tapped twice until tapped again         |

One-shot keys are useful for modifiers and layers, e.g. with `leftshift: one-shot leftshift`, tapping `leftshift` and
then `a` results in `A`.

Another option to trigger actions is via key combos, e.g. `f+d: layer mouse`, which is triggered when `f` and `d` are
pressed simultaneously. The maximum duration between the presses is defined with the `comboTime` config option.

Pressing `esc` always returns to the initial layer (if not already there), which is helpful if you get stuck or are
unsure of the current layer. To disable this behaviour for a specific layer, you can explicitly map the key,
e.g., `esc: esc`.

## Custom devices

If you don't want mouseless to read from all keyboards, you can specify one or more devices in the configuration file.

To find available devices, you can use the `--list-devices` flag:

```sh
sudo mouseless --list-devices
```

This will show all keyboard devices with their names and some other information. In case your keyboard is not detected,
you can use the `--list-all-devices` flag to show all input devices, regardless of whether mouseless thinks it is a
keyboard.

In the config file, you can then specify the name of one or more devices, e.g.:

```yaml
devices:
- "Some keyboard name"
- "Some other keyboard"
```

If you instead want to exclude specific devices, you can use the `devicesExclude` option.

## Run without sudo

To run mouseless without root privileges, you need to give your user permission to read from keyboard devices and to
create virtual input devices.

> **Note:** Doing so gives all applications running under your user the ability to read from your keyboards,
> so this does have some security implications. If you want to avoid this, you can create a separate user for running
> mouseless only and give that user the necessary permissions instead.

First make sure that the uinput group exists and add your user to the `input` and `uinput` groups:

```sh
sudo groupadd --system uinput
sudo usermod -a -G input,uinput $USER
```

Then add a udev rule so that users in the `uinput` group can create uinput devices:

```sh
sudo tee /etc/udev/rules.d/99-mouseless.rules <<EOF
KERNEL=="uinput", GROUP="uinput", MODE="0660"
EOF
```

To apply the changes, you can simply reboot your machine.

If it still doesn’t work, the uinput kernel module might not be loaded, which you can load manually with:

```sh
sudo modprobe uinput
```

To load the uinput module automatically at boot, create this file:

```sh
echo "uinput" | sudo tee /etc/modules-load.d/uinput.conf
```

## Run at startup with systemd

One option to automatically start mouseless at startup is using `systemd`, which is available in most distros.

### With root privileges

Create a service file (replace the config file path, and the path to the mouseless binary if necessary):

```sh
sudo tee /etc/systemd/system/mouseless.service <<EOF
[Unit]
Description=mouseless

[Service]
Type=simple
ExecStart=/usr/local/bin/mouseless --config /path/to/config.yaml

[Install]
WantedBy=multi-user.target
EOF
```

Enable and start the service:

```sh
sudo systemctl enable mouseless.service
sudo systemctl start mouseless.service
```

You can check the status with:

```sh
sudo systemctl status mouseless.service
```

### Without root privileges

You can also install mouseless for a specific user only (the user needs to have permission to run mouseless, see
section `Run without sudo`).

Create a service file (replace the config file path):

```sh
tee "$HOME/.config/systemd/user/mouseless.service" <<EOF
[Unit]
Description=mouseless

[Service]
Type=simple
ExecStart=$HOME/.local/bin/mouseless --config /path/to/config.yaml

[Install]
WantedBy=default.target
EOF
```

Enable and start the service:

```sh
systemctl --user daemon-reload
systemctl --user enable mouseless.service
systemctl --user start mouseless.service
```

You can check the status with:

```sh
systemctl status --user mouseless.service
```

## Pointer acceleration

mouseless emits mouse events through a virtual mouse device, which means that your desktop environment applies its
usual pointer acceleration on top, just like for a physical mouse. As a result, the effective pointer speed grows more
than linearly: a `speed 2.0` binding moves the pointer more than twice as fast, and moving diagonally feels faster than
expected.

If you prefer a linear response, you can disable the acceleration for the virtual mouse only, so that your physical
mouse or touchpad is not affected. The virtual device is named `mouseless mouse` with vendor id `18193` and product
id `2070` (if you set the `instanceName` config option, the name changes accordingly).

How to disable the acceleration depends on your environment:

**river** (in `~/.config/river/init`):

```sh
riverctl input pointer-18193-2070-mouseless_mouse accel-profile flat
```

**sway** (in `~/.config/sway/config`):

```
input "18193:2070:mouseless_mouse" accel_profile flat
```

**Hyprland** (in `~/.config/hypr/hyprland.lua`):

```lua
hl.device({
    name = "mouseless-mouse",
    accel_profile = "flat",
})
```

**KDE Plasma**: with mouseless running, open System Settings → Mouse & Touchpad, select the device `mouseless mouse`
and set the pointer acceleration profile to flat.

**GNOME**: unfortunately GNOME has no per-device settings, so this affects physical mice as well:

```sh
gsettings set org.gnome.desktop.peripherals.mouse accel-profile 'flat'
```

**X11**:

```sh
xinput set-prop "mouseless mouse" "libinput Accel Profile Enabled" 0, 1
```

Note that Wayland compositors apply the setting automatically whenever the virtual device appears, so it does not
matter whether mouseless is running yet. With `xinput` the device has to exist, so the command has to be run after
mouseless has started, for example with the `startCommand` config option:

```yaml
startCommand: xinput set-prop "mouseless mouse" "libinput Accel Profile Enabled" 0, 1
```

This only works if mouseless runs as your own user, because when started with `sudo`, the command cannot connect to your
display session.

Since the pointer now moves slower than before, you might want to increase `baseMouseSpeed` and adjust the `speed`
values to compensate.

## Disable touchpad while typing

Many desktop environments can disable the touchpad while typing, but this only works for keyboards that libinput
considers to be built in. Since all key events come from the virtual keyboard of mouseless, which looks like an
external keyboard, the touchpad stays active.

You can fix this by telling libinput to treat the virtual keyboard as an internal one, by creating the file
`/etc/libinput/local-overrides.quirks` (if you set the `instanceName` config option, adjust the name accordingly):

```ini
[mouseless keyboard]
MatchUdevType=keyboard
MatchName=mouseless keyboard
AttrKeyboardIntegration=internal
```

The file is only read when libinput starts, so you have to log out and back in (or reboot) for it to take effect.
