# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
