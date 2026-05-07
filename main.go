package main

import (
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"kaijuengine.com/bootstrap"
	"kaijuengine.com/engine"
	"kaijuengine.com/engine/assets"
	"kaijuengine.com/engine/cameras"
	"kaijuengine.com/engine/physics"
	"kaijuengine.com/engine/systems/visual2d/sprite"
	"kaijuengine.com/engine/ui"
	"kaijuengine.com/matrix"
	"kaijuengine.com/platform/hid"
	"kaijuengine.com/rendering"
)

const (
	windowWidth  = 640
	windowHeight = 480
	spriteSize   = 32
	wallDepth    = 32

	defaultObjects = 100
	objectStep     = 100
	minObjects     = 0
	maxObjects     = 5000
	maxSpawnFrame  = 25

	gameContentPath      = "game_content"
	kaijuEngineEnv       = "KAIJU_ENGINE_DIR"
	defaultKaijuSrcPath  = "../kaiju/src"
	rawEngineContentPath = "editor/editor_embedded_content/editor_content"
)

type benchmarkObject struct {
	sprite *sprite.Sprite
	shape  *physics.BoxShape
	motion *physics.MotionState
	body   *physics.RigidBody
}

type wallBody struct {
	entity *engine.Entity
	shape  *physics.BoxShape
	motion *physics.MotionState
	body   *physics.RigidBody
}

type Game struct {
	host          *engine.Host
	uiManager     ui.Manager
	menuRoot      *ui.Panel
	overlayLabel  *ui.Label
	objects       []benchmarkObject
	walls         []wallBody
	targetCount   int
	updateID      engine.UpdateId
	rng           *rand.Rand
	smoothedFPS   float64
	uiTimer       float64
	benchmarkOn   bool
	lastObjectLog int
}

func main() {
	bootstrap.Main(getGame(), nil)
}

func getGame() bootstrap.GameInterface {
	return &Game{
		targetCount:   defaultObjects,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		lastObjectLog: -1,
	}
}

func (Game) PluginRegistry() []reflect.Type {
	return []reflect.Type{}
}

func (Game) ContentDatabase() (assets.Database, error) {
	if _, err := os.Stat(gameContentPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := copyKaijuStockContent(); err != nil {
			return nil, err
		}
	} else {
		logGame("using existing game content", "path", gameContentPath)
	}
	return assets.NewFileDatabase(gameContentPath)
}

func (g *Game) Launch(host *engine.Host) {
	logGame("launching game")
	g.host = host
	g.configureWindowAndCameras()
	g.configurePhysics()
	g.uiManager.Init(g.host)
	g.createMainMenu()
	g.createOverlay()

	g.updateID = host.Updater.AddUpdate(g.update)
	host.OnClose.Add(func() {
		host.Updater.RemoveUpdate(&g.updateID)
	})
}

func (g *Game) configureWindowAndCameras() {
	g.host.Window.SetTitle("Kaiju 2D Benchmark")
	g.host.Window.SetSize(windowWidth, windowHeight)

	primary := cameras.NewStandardCameraOrthographic(
		windowWidth, windowHeight,
		windowWidth, windowHeight,
		matrix.NewVec3(0, 0, 250),
	)
	primary.SetLookAt(matrix.Vec3Zero())
	g.host.Cameras.Primary.ChangeCamera(primary)

	uiCamera := cameras.NewStandardCameraOrthographic(
		windowWidth, windowHeight,
		windowWidth, windowHeight,
		matrix.NewVec3(0, 0, 250),
	)
	uiCamera.SetLookAt(matrix.Vec3Zero())
	g.host.Cameras.UI.ChangeCamera(uiCamera)
}

func (g *Game) configurePhysics() {
	g.host.StartPhysics()
	g.host.Physics().World().SetGravity(matrix.Vec3Zero())
}

func (g *Game) createOverlay() {
	g.overlayLabel = g.uiManager.Add().ToLabel()
	g.overlayLabel.Init("")
	g.overlayLabel.SetFontSize(15)
	g.overlayLabel.SetColor(matrix.ColorWhite())
	g.overlayLabel.SetBGColor(matrix.ColorTransparent())
	g.overlayLabel.SetWrap(false)
	g.overlayLabel.SetMaxWidth(windowWidth - 16)
	g.overlayLabel.Base().Layout().Scale(windowWidth-16, 64)
	g.overlayLabel.Base().Layout().SetOffset(8, 8)
	g.refreshOverlay()
	g.overlayLabel.Hide()
}

func (g *Game) createMainMenu() {
	logGame("showing main menu")
	g.menuRoot = g.uiManager.Add().ToPanel()
	g.menuRoot.Init(nil, ui.ElementTypePanel)
	g.menuRoot.DontFitContent()
	g.menuRoot.SetColor(matrix.ColorRGBAInt(14, 18, 24, 255))
	g.menuRoot.Base().Layout().Scale(windowWidth, windowHeight)
	g.menuRoot.Base().Layout().SetOffset(0, 0)

	title := g.uiManager.Add().ToLabel()
	title.Init("Kaiju 2D Benchmark")
	title.SetFontSize(28)
	title.SetColor(matrix.ColorWhite())
	title.SetBGColor(matrix.ColorTransparent())
	title.SetJustify(rendering.FontJustifyCenter)
	title.SetBaseline(rendering.FontBaselineCenter)
	title.SetWrap(false)
	title.SetMaxWidth(420)
	title.Base().Layout().Scale(420, 56)
	title.Base().Layout().SetOffset(110, 96)
	g.menuRoot.AddChild(title.Base())

	start := g.createMenuButton("Start Benchmark", 220, 194, func() {
		logGame("start benchmark selected")
		g.startBenchmark()
	})
	g.menuRoot.AddChild(start.Base())

	quit := g.createMenuButton("Quit", 220, 260, func() {
		logGame("quit selected from main menu")
		g.host.Close()
	})
	g.menuRoot.AddChild(quit.Base())
}

func (g *Game) createMenuButton(text string, x, y float32, click func()) *ui.Button {
	tex, _ := g.host.TextureCache().Texture(assets.TextureSquare, rendering.TextureFilterLinear)
	button := g.uiManager.Add().ToButton()
	button.Init(tex, text)
	button.Label().SetText(text)
	button.Label().SetFontSize(18)
	button.SetColor(matrix.ColorRGBAInt(230, 234, 241, 255))
	button.Base().Layout().Scale(200, 48)
	button.Base().Layout().SetOffset(x, y)
	button.Base().AddEvent(ui.EventTypeClick, click)
	return button
}

func (g *Game) startBenchmark() {
	if g.benchmarkOn {
		return
	}
	g.benchmarkOn = true
	g.lastObjectLog = -1
	logGame("starting benchmark", "targetObjects", g.targetCount)
	if g.menuRoot != nil {
		g.host.DestroyEntity(g.menuRoot.Base().Entity())
		g.menuRoot = nil
	}
	g.overlayLabel.Show()
	g.createWalls()
	g.refreshOverlay()
}

func (g *Game) createWalls() {
	logGame("creating wall colliders")
	g.walls = append(g.walls,
		g.createWall("left", -windowWidth/2-wallDepth/2, 0, wallDepth, windowHeight+wallDepth*2),
		g.createWall("right", windowWidth/2+wallDepth/2, 0, wallDepth, windowHeight+wallDepth*2),
		g.createWall("top", 0, windowHeight/2+wallDepth/2, windowWidth+wallDepth*2, wallDepth),
		g.createWall("bottom", 0, -windowHeight/2-wallDepth/2, windowWidth+wallDepth*2, wallDepth),
	)
}

func (g *Game) createWall(name string, x, y, width, height int) wallBody {
	entity := engine.NewEntity(g.host.WorkGroup())
	entity.SetName("wall_" + name)
	entity.Transform.SetPosition(matrix.NewVec3(matrix.Float(x), matrix.Float(y), 0))
	entity.Transform.SetScale(matrix.NewVec3(matrix.Float(width), matrix.Float(height), 1))

	shape := physics.NewBoxShape(matrix.NewVec3(matrix.Float(width)/2, matrix.Float(height)/2, 0.5))
	motion := physics.NewDefaultMotionState(matrix.QuaternionIdentity(), entity.Transform.Position())
	body := physics.NewRigidBody(0, motion, &shape.CollisionShape, matrix.Vec3Zero())
	g.host.Physics().AddEntity(entity, body)

	return wallBody{entity: entity, shape: shape, motion: motion, body: body}
}

func (g *Game) update(deltaTime float64) {
	g.enforceFixedResolution()
	if !g.benchmarkOn {
		g.handleMenuKeyboard()
		return
	}
	g.handleKeyboard()
	g.syncObjectCount()
	g.updateFPS(deltaTime)
}

func (g *Game) enforceFixedResolution() {
	if g.host.Window.Width() != windowWidth || g.host.Window.Height() != windowHeight {
		g.host.Window.SetSize(windowWidth, windowHeight)
		g.host.Cameras.Primary.Camera.ViewportChanged(windowWidth, windowHeight)
		g.host.Cameras.UI.Camera.ViewportChanged(windowWidth, windowHeight)
	}
}

func (g *Game) handleMenuKeyboard() {
	keyboard := g.host.Window.Keyboard
	if keyboard.KeyDown(hid.KeyboardKeyReturn) || keyboard.KeyDown(hid.KeyboardKeyEnter) {
		logGame("start benchmark selected from keyboard")
		g.startBenchmark()
	} else if keyboard.KeyDown(hid.KeyboardKeyQ) {
		logGame("quit selected from keyboard")
		g.host.Close()
	}
}

func (g *Game) handleKeyboard() {
	keyboard := g.host.Window.Keyboard
	switch {
	case keyboard.KeyDown(hid.KeyboardKeyA):
		g.setTargetCount(g.targetCount+objectStep, "increase target")
	case keyboard.KeyDown(hid.KeyboardKeyZ):
		g.setTargetCount(g.targetCount-objectStep, "decrease target")
	case keyboard.KeyDown(hid.KeyboardKeyR):
		g.setTargetCount(defaultObjects, "reset target")
	case keyboard.KeyDown(hid.KeyboardKeyQ):
		logGame("quit selected during benchmark")
		g.host.Close()
	}
}

func (g *Game) setTargetCount(target int, reason string) {
	next := clampInt(target, minObjects, maxObjects)
	if next == g.targetCount {
		return
	}
	g.targetCount = next
	logGame(reason, "targetObjects", g.targetCount)
}

func (g *Game) syncObjectCount() {
	spawned := 0
	for len(g.objects) < g.targetCount && spawned < maxSpawnFrame {
		g.objects = append(g.objects, g.spawnObject())
		spawned++
	}
	for len(g.objects) > g.targetCount {
		idx := len(g.objects) - 1
		obj := g.objects[idx]
		g.host.DestroyEntity(&obj.sprite.Entity)
		g.objects = g.objects[:idx]
	}
	g.logObjectProgress()
}

func (g *Game) spawnObject() benchmarkObject {
	x := randomRange(g.rng, -windowWidth/2+spriteSize, windowWidth/2-spriteSize)
	y := randomRange(g.rng, -windowHeight/2+spriteSize, windowHeight/2-spriteSize)

	color := matrix.NewColor(
		matrix.Float(randomRange(g.rng, 90, 245))/255,
		matrix.Float(randomRange(g.rng, 90, 245))/255,
		matrix.Float(randomRange(g.rng, 90, 245))/255,
		1,
	)

	s := &sprite.Sprite{}
	s.Init(float32(x), float32(y), spriteSize, spriteSize, g.host, assets.TextureSquare, color)
	s.SetUnBlended()

	shape := physics.NewBoxShape(matrix.NewVec3(spriteSize/2, spriteSize/2, 0.5))
	motion := physics.NewDefaultMotionState(matrix.QuaternionIdentity(), s.Entity.Transform.Position())
	inertia := shape.CollisionShape.CalculateLocalInertia(1)
	body := physics.NewRigidBody(1, motion, &shape.CollisionShape, inertia)
	g.host.Physics().AddEntity(&s.Entity, body)

	impulse := matrix.NewVec3(
		matrix.Float(randomRange(g.rng, -240, 240)),
		matrix.Float(randomRange(g.rng, -240, 240)),
		0,
	)
	if impulse.LengthSquared() < 1 {
		impulse = matrix.NewVec3(120, 80, 0)
	}
	body.ApplyImpulseAtPoint(impulse, s.Entity.Transform.Position())

	return benchmarkObject{sprite: s, shape: shape, motion: motion, body: body}
}

func (g *Game) updateFPS(deltaTime float64) {
	if deltaTime > 0 {
		fps := 1.0 / deltaTime
		if g.smoothedFPS == 0 {
			g.smoothedFPS = fps
		} else {
			g.smoothedFPS = g.smoothedFPS*0.9 + fps*0.1
		}
	}

	g.uiTimer += deltaTime
	if g.uiTimer >= 0.25 {
		g.uiTimer = 0
		g.refreshOverlay()
	}
}

func (g *Game) refreshOverlay() {
	if g.overlayLabel == nil {
		return
	}
	g.overlayLabel.SetText(fmt.Sprintf(
		"FPS: %.0f\nObjects: %d/%d\nA add | Z remove | R reset | Q quit",
		g.smoothedFPS,
		len(g.objects),
		g.targetCount,
	))
}

func copyKaijuStockContent() error {
	sourceRoot := kaijuSourceRoot()
	rawPath := filepath.Join(sourceRoot, filepath.FromSlash(rawEngineContentPath))
	if _, err := os.Stat(rawPath); err != nil {
		return fmt.Errorf("Kaiju stock content was not found at %q; clone Kaiju with submodules or set %s", rawPath, kaijuEngineEnv)
	}

	logGame("copying Kaiju stock content to the project database", "source", rawPath, "target", gameContentPath)
	if err := os.MkdirAll(gameContentPath, os.ModePerm); err != nil {
		return err
	}

	top, err := os.ReadDir(rawPath)
	if err != nil {
		return err
	}

	all := []string{}
	var readSubDir func(path string) error
	readSubDir = func(path string) error {
		if strings.HasSuffix(filepath.ToSlash(path), "renderer/src") {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for i := range entries {
			subPath := filepath.Join(path, entries[i].Name())
			if entries[i].IsDir() {
				if err := readSubDir(subPath); err != nil {
					return err
				}
				continue
			}
			all = append(all, subPath)
		}
		return nil
	}

	skip := []string{"editor", "meshes"}
	for i := range top {
		if !top[i].IsDir() || slices.Contains(skip, top[i].Name()) {
			continue
		}
		if err := readSubDir(filepath.Join(rawPath, top[i].Name())); err != nil {
			return err
		}
	}

	for i := range all {
		outPath := filepath.Join(gameContentPath, filepath.Base(all[i]))
		data, err := os.ReadFile(all[i])
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, data, os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) logObjectProgress() {
	count := len(g.objects)
	if count == g.lastObjectLog {
		return
	}
	if count == g.targetCount || count == 0 || count%100 == 0 {
		g.lastObjectLog = count
		logGame("object count updated", "current", count, "target", g.targetCount)
	}
}

func logGame(message string, args ...any) {
	slog.Info(message, args...)
	if len(args) == 0 {
		fmt.Printf("[GameBench] %s\n", message)
		return
	}
	fmt.Printf("[GameBench] %s", message)
	for i := 0; i+1 < len(args); i += 2 {
		fmt.Printf(" %v=%v", args[i], args[i+1])
	}
	fmt.Println()
}

func kaijuSourceRoot() string {
	if path := os.Getenv(kaijuEngineEnv); path != "" {
		return path
	}
	return defaultKaijuSrcPath
}

func randomRange(rng *rand.Rand, min, max int) int {
	if max <= min {
		return min
	}
	return min + rng.Intn(max-min+1)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
