# GameBench.Kaijue

A small Kaiju Engine desktop benchmark. It opens a fixed `640x480` window, shows a main menu, then spawns `32x32` floating sprites with box physics bodies, keeps them inside wall colliders, and shows current FPS.

## Prerequisites

- Go 1.25 or newer.
- Git.
- A working C/C++ toolchain for CGo. On Windows, install MinGW-w64 and make sure `gcc` and `g++` are on `PATH`.
- Vulkan runtime and GPU drivers.
- Kaiju Engine cloned with submodules.

The default project layout expects Kaiju next to this repo:

```text
C:\Projects
  kaiju\
    src\
  GameBench.Kaijue\
```

Clone Kaiju with:

```powershell
cd C:\Projects
git clone --recurse-submodules https://github.com/KaijuEngine/kaiju.git
```

The local dependency path is configured in `go.mod`:

```go
replace kaijuengine.com => ../kaiju/src
```

If Kaiju is somewhere else, update that `replace` path. At runtime, the game also copies Kaiju's stock engine content into `game_content` on first launch. If the engine source is not at `../kaiju/src`, set `KAIJU_ENGINE_DIR` to the Kaiju `src` directory:

```powershell
$env:KAIJU_ENGINE_DIR = "D:\dev\kaiju\src"
```

## Build And Run

From this repo:

```powershell
cd C:\Projects\GameBench.Kaijue
$env:CGO_ENABLED = "1"
go mod tidy
go run .
```

To build an executable:

```powershell
$env:CGO_ENABLED = "1"
go build -ldflags '-linkmode external -extldflags "-static -static-libgcc -static-libstdc++"' -o GameBench.Kaijue.exe .
.\GameBench.Kaijue.exe
```

If Windows reports a missing MinGW DLL such as `libgcc_s_seh-1.dll`, `libstdc++-6.dll`, or `libwinpthread-1.dll`, rebuild with the static command above. As an alternative for local development, add `C:\msys64\mingw64\bin` to `PATH` before launching:

```powershell
$env:PATH = "C:\msys64\mingw64\bin;$env:PATH"
.\GameBench.Kaijue.exe
```

On first run, `game_content` is created from Kaiju's stock content. It is ignored by Git.

## Controls

Main menu:

- Click `Start Benchmark`, or press `Enter`, to start.
- Click `Quit`, or press `Q`, to quit.

Benchmark:

- `A`: add objects.
- `Z`: remove objects.
- `R`: reset to the default object count.
- `Q`: quit.

Defaults: `100` objects, step size `100`, minimum `0`, maximum `5000`.
