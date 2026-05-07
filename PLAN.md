# Kaiju 2D Benchmark Game Plan

## Summary
Build a small desktop benchmark game in this repo that uses an external local Kaiju Engine checkout. The game runs at fixed `640x480`, renders many `32x32` sprites, gives each sprite a box physics body, disables gravity, contains objects with wall colliders, and shows an FPS/object-count overlay.

## Key Changes
- Create a Go module in this repo with `require kaijuengine.com` and `replace kaijuengine.com => ../kaiju/src`.
- Implement a runtime-only Kaiju game with `main.go`, `bootstrap.Main`, and a `Game` type implementing Kaiju's `GameInterface`.
- Add a main menu scene with `Start Benchmark` and `Quit`; the benchmark objects are not spawned until the benchmark starts.
- Use `assets.NewFileDatabase("game_content")`, copying Kaiju's stock runtime content from the external checkout on first run.
- Force the window to `640x480`, configure centered orthographic cameras, start physics, and set gravity to `(0, 0, 0)`.
- Spawn `32x32` sprite objects with matching Bullet box rigid bodies, random starting positions, and initial impulses.
- Add four static wall colliders around the viewport.
- Add keyboard controls:
  - Main menu: `Enter` starts the benchmark, `Q` quits.
  - `A`: increase target object count.
  - `Z`: decrease target object count.
  - `R`: reset to default count.
  - `Q`: quit the game.
- Add a UI overlay showing FPS, object count, and compact controls.
- Add `README.md` explaining prerequisites, Kaiju clone layout, build/run commands, controls, and benchmark behavior.

## Test Plan
- With Kaiju cloned at `../kaiju/src`, ensure `CGO_ENABLED=1` and a C/C++ compiler such as MinGW-w64 is on `PATH`.
- Run `go mod tidy`.
- Run `go build .`.
- Launch the game and verify:
  - Window opens at `640x480`.
  - FPS/object labels update.
  - Objects are `32x32`, float with no gravity, collide with each other, and stay inside walls.
  - `A`, `Z`, `R`, and `Q` work.
- Stress-check object counts at `0`, `100`, `1000`, and `5000`.

## Assumptions
- Kaiju Engine is not vendored into this repo.
- Default Kaiju location is `../kaiju/src`.
- The local `replace` is intentional because Kaiju's module path is `kaijuengine.com` and its source-build workflow depends on a cloned repo with submodules/native libraries.
