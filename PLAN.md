# Kaiju 2D Benchmark Game Plan

## Summary
Build a small desktop benchmark game in this repo that uses an external local Kaiju Engine checkout. The game runs at fixed `1280x960`, renders many animated `32x32` sprites with custom 2D box collision, disables gravity, contains objects within viewport bounds, and shows an FPS/object-count overlay.

## Key Changes
- Create a Go module in this repo with `require kaijuengine.com` and `replace kaijuengine.com => ../kaiju/src`.
- Implement a runtime-only Kaiju game with `main.go`, manual host startup, and a `Game` type for launch/update behavior.
- Add a main menu scene with `Start Benchmark` and `Quit`; the benchmark objects are not spawned until the benchmark starts.
- Use `assets.NewFileDatabase("game_content")`, copying Kaiju's stock runtime content from the external checkout on first run.
- Force the window to `1280x960`, configure centered orthographic cameras for pixel-perfect 2D rendering, start physics, and set gravity to `(0, 0, 0)`.
- Spawn animated `32x32` pixel-art sprite objects with 2D box collision and random starting velocities.
- Load sprite frames with nearest-neighbor filtering and snap sprite render positions to whole pixels.
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
  - Window opens at `1280x960`.
  - FPS/object labels update.
  - Objects are `32x32`, render pixel-perfect, float with no gravity, collide with each other, and stay inside bounds.
  - `A`, `Z`, `R`, and `Q` work.
- Stress-check object counts at `0`, `100`, `1000`, and `5000`.

## Assumptions
- Kaiju Engine is not vendored into this repo.
- Default Kaiju location is `../kaiju/src`.
- The local `replace` is intentional because Kaiju's module path is `kaijuengine.com` and its source-build workflow depends on a cloned repo with submodules/native libraries.
