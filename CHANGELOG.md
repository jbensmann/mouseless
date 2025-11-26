# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - Unreleased

### Added

- Support high-resolution scrolling (applications need to support it).
- Devices can now also be specified by their name.
- New config option `devicesExclude` to ignore specific devices (#67).
- New flag `--list-devices` to list available input devices.
- New config option `instanceName` to support running multiple instances.
- New action `mod-layer` to overload a modifier key (#48).
- New action `exec-press-release` to execute different commands on key press and release (#74).

### Changed

- Improved hotplug support for keyboards (thanks to @h43z).
- Don't exit if there are no matching devices at startup.
- Log warnings for unknown or duplicate keys in the config file (#96).

### Fixed

- Disable early-release in tap-hold for modifier keys (#95).

## [0.2.0] - 2024-10-19

### Added

- It is now possible to map an action to a combo of two keys (#42).
- New config option`quickTapTime` to hold the tap key when pressed twice.
- New config option`mouseLoopInterval` to change the rate at which the mouse pointer moves.
- One can specify commands that are executed when a layer is entered/exited with `enterCommand`/`exitCommand` (#53).

### Changed

- When `esc` is mapped, mouseless does not go to the initial layer when pressed (#44).

### Fixed

- The middle mouse button was always released immediately.
