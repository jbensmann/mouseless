# mouseless

This program lets you control the mouse pointer in Linux using the keyboard. It works across all Linux distributions,
including those running Wayland.

It is the successor to [xmouseless](https://github.com/jbensmann/xmouseless).

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

### Binary Release

You can download a precompiled binary from [Releases](https://github.com/jbensmann/mouseless/releases).

The file `mouseless-linux-amd64.tar.gz` contains the executable named `mouseless`. Put it in an appropriate location and
make it executable, e.g.:

```sh
tar -xvf mouseless-linux-amd64.tar.gz
sudo mv dist/mouseless /usr/local/bin/mouseless
sudo chmod +x /usr/local/bin/mouseless
```

Alternatively, you can use this convenience script to install the latest release and also create a config file if none
exists yet:

```sh
wget -O install.sh https://raw.githubusercontent.com/jbensmann/mouseless/refs/heads/main/install.sh
sh install.sh
```

> **Note:** You should always check the content of scripts from untrusted sources before executing them.

### From Source

You can download and build mouseless directly with:

```shell
go install github.com/jbensmann/mouseless@latest
```

This places the binary in `$GOPATH/bin/mouseless` (usually `~/go/bin/mouseless`):

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

> **Note:** After starting, release all keys and wait one second before typing. Otherwise, keys might get stuck in the
> pressed state. If that happens, stop mouseless and press the stuck key before starting again.


For troubleshooting, you can use the --debug flag to show more verbose log messages.

## Configuration

The configuration file is in YAML format, you do not need to know exactly what that means, just make sure
that the indentation level of the lines is correct.
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
| `exec <cmd>`                        | `exec notify-send "hello from mouseless"`                   | executes the given command (the example sends a desktop notification)                               |
| `exec-press-release <cmd1>; <cmd2>` | `exec-press-release notify-send press; notify-send release` | executes different commands when the key is pressed and released                                    |
| `reload-config`                     | `reload-config`                                             | reloads the configuration file                                                                      |

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

Another option to trigger actions is via key combos, e.g. `f+d: layer mouse`, which is triggered when `f` and `d` are
pressed simultaneously. The maximum duration between the presses is defined with the `comboTime` config option.

Pressing `esc` always returns to the initial layer (if not already there), which is helpful if one gets stuck or is
unsure of the current layer. To disable this behaviour for a specific layer, you can explicitly map the key,
e.g., `esc: esc`.

## Custom devices

If you don't want mouseless to read from all keyboards, you can specify one or more devices in the configuration file.

To find available devices, you can use the `--list-devices` flag:

```sh
sudo mouseless --list-devices
```

This will show all keyboard devices with their names and some other information. In case your keyboard is not detected,
you can use the `--list-all-devices` flag to show all input device, regardless of whether mouseless thinks it is a
keyboard or not.

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

If it still doesn’t work, the uinput kernel module might not be loaded, which you can do manually with:

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

1. Download the latest release of mouseless, e.g. to `/usr/local/bin/mouseless`, and make it executable,
   e.g. `sudo chmod +x /usr/local/bin/mouseless`.
2. Create a service file (replace the config file path):
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
3. Enable and start the service:
   ```sh
   sudo systemctl enable mouseless.service
   sudo systemctl start mouseless.service
   ```
   You can check the status with:
   ```sh
   sudo systemctl status mouseless.service
   ```

### Without root privileges

One can also install mouseless for a specific user only (the user needs to have permission to run mouseless, see
section `Run without sudo`):

1. Download the latest release of mouseless, e.g. to `$HOME/.local/bin/mouseless`, and make it executable,
   e.g. `chmod +x $HOME/.local/bin/mouseless`.
2. Create a service file (replace the config file path):
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
3. Enable and start the service:
   ```sh
   systemctl --user daemon-reload
   systemctl --user enable mouseless.service
   systemctl --user start mouseless.service
   ```
   You can check the status with:
   ```sh
   systemctl status --user mouseless.service
   ```
