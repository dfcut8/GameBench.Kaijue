package main

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"kaijuengine.com/engine"
	"kaijuengine.com/engine/assets"
	"kaijuengine.com/engine/cameras"
	"kaijuengine.com/engine/host_container"
	"kaijuengine.com/engine/physics"
	"kaijuengine.com/engine/systems/logging"
	"kaijuengine.com/engine/systems/visual2d/sprite"
	"kaijuengine.com/engine/ui"
	"kaijuengine.com/matrix"
	"kaijuengine.com/platform/hid"
	"kaijuengine.com/rendering"
)

const (
	windowWidth  = 1280
	windowHeight = 960
	spriteSize   = 32
	wallDepth    = 32

	defaultObjects = 100
	objectStep     = 100
	minObjects     = 0
	maxObjects     = 5000
	maxSpawnFrame  = 25
	flashDuration  = 0.25

	gameContentPath      = "game_content"
	customContentPath    = "assets"
	kaijuEngineEnv       = "KAIJU_ENGINE_DIR"
	defaultKaijuSrcPath  = "../kaiju/src"
	rawEngineContentPath = "editor/editor_embedded_content/editor_content"
	musicKey             = "audio/benchmark_loop.wav"
	uiFontFace           = rendering.FontFace("fonts/Kenney Pixel Square")
)

var benchmarkFrameKeys = []string{
	"sprites/bench_blob_0.png",
	"sprites/bench_blob_1.png",
	"sprites/bench_blob_2.png",
	"sprites/bench_blob_3.png",
	"sprites/bench_blob_4.png",
	"sprites/bench_blob_5.png",
	"sprites/bench_blob_6.png",
	"sprites/bench_blob_7.png",
}

type benchmarkObject struct {
	sprite     *sprite.Sprite
	baseColor  matrix.Color
	x          float64
	y          float64
	vx         float64
	vy         float64
	frame      int
	frameTimer float64
	flashTimer float64
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
	overlayRoot   *ui.Panel
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
	animation     []*rendering.Texture
}

func main() {
	if err := runGame(); err != nil {
		logGame("failed to run game", "error", err)
		os.Exit(1)
	}
}

func getGame() *Game {
	return &Game{
		targetCount:   defaultObjects,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		lastObjectLog: -1,
	}
}

func runGame() error {
	logStream := logging.Initialize(nil)
	defer logStream.Close()

	game := getGame()
	adb, err := game.ContentDatabase()
	if err != nil {
		return err
	}

	container := host_container.New("Kaiju 2D Benchmark", logStream, adb)
	container.RunFunction(func() {
		container.Host.Window.EnableRawMouseInput()
		game.Launch(container.Host)
	})

	go func() {
		if err := container.Run(windowWidth, windowHeight, -1, -1, nil); err != nil {
			logGame("host run failed", "error", err)
		}
	}()

	<-container.PrepLock
	<-container.Host.Done()
	return nil
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
	if err := copyCustomContent(); err != nil {
		return nil, err
	}
	return assets.NewFileDatabase(gameContentPath)
}

func (g *Game) Launch(host *engine.Host) {
	logGame("launching game")
	g.host = host
	logGame("configuring cameras")
	g.configureWindowAndCameras()
	logGame("configuring physics")
	g.configurePhysics()
	g.playBackgroundMusic()
	logGame("initializing ui")
	g.uiManager.Init(g.host)
	g.createMainMenu()
	g.createOverlay()

	g.updateID = host.Updater.AddUpdate(g.update)
	lateUpdateID := host.LateUpdater.AddUpdate(g.lateUpdate)
	host.OnClose.Add(func() {
		host.Updater.RemoveUpdate(&g.updateID)
		host.LateUpdater.RemoveUpdate(&lateUpdateID)
	})
}

func (g *Game) configureWindowAndCameras() {
	primary := cameras.NewStandardCameraOrthographic(windowWidth, windowHeight, windowWidth, windowHeight, matrix.NewVec3(0, 0, 100))
	primary.SetProperties(60, -500, 500, windowWidth, windowHeight)
	primary.SetLookAt(matrix.Vec3Zero())
	g.host.Cameras.Primary.ChangeCamera(primary)

	uiCamera := cameras.NewStandardCameraOrthographic(windowWidth, windowHeight, windowWidth, windowHeight, matrix.NewVec3(0, 0, 250))
	uiCamera.SetProperties(60, -500, 500, windowWidth, windowHeight)
	uiCamera.SetLookAt(matrix.Vec3Zero())
	g.host.Cameras.UI.ChangeCamera(uiCamera)

	logGame("configured 2d camera", "width", g.host.Window.Width(), "height", g.host.Window.Height())
}

func (g *Game) configurePhysics() {
	g.host.StartPhysics()
	g.host.Physics().World().SetGravity(matrix.Vec3Zero())
}

func (g *Game) playBackgroundMusic() {
	if g.host.Audio() == nil {
		logGame("audio system unavailable")
		return
	}
	g.host.Audio().SetMusicVolume(0.18)
	clip, err := g.host.Audio().LoadMusic(g.host.AssetDatabase(), musicKey)
	if err != nil {
		logGame("failed to load background music", "key", musicKey, "error", err)
		return
	}
	_, handle := g.host.Audio().PlayMusic(musicKey)
	if handle == 0 {
		logGame("failed to start background music", "key", musicKey)
		return
	}
	logGame("playing background music", "key", musicKey, "seconds", fmt.Sprintf("%.1f", clip.Length()))
}

func (g *Game) createOverlay() {
	const (
		overlayWidth  = 390
		overlayHeight = 104
		overlayPad    = 10
	)
	tex, _ := g.host.TextureCache().Texture(assets.TextureSquare, rendering.TextureFilterNearest)
	g.overlayRoot = g.uiManager.Add().ToPanel()
	g.overlayRoot.Init(tex, ui.ElementTypePanel)
	g.overlayRoot.DontFitContent()
	g.overlayRoot.SetColor(matrix.ColorRGBAInt(8, 10, 14, 190))
	g.overlayRoot.AllowClickThrough()
	g.overlayRoot.Base().Layout().Scale(overlayWidth, overlayHeight)
	g.overlayRoot.Base().Layout().SetOffset(8, 8)

	g.overlayLabel = g.uiManager.Add().ToLabel()
	g.overlayLabel.Init("")
	g.overlayLabel.SetFontFace(uiFontFace)
	g.overlayLabel.SetFontSize(17)
	g.overlayLabel.SetColor(matrix.ColorWhite())
	g.overlayLabel.SetBGColor(matrix.ColorTransparent())
	g.overlayLabel.SetWrap(false)
	g.overlayLabel.SetMaxWidth(overlayWidth - overlayPad*2)
	g.overlayLabel.Base().Layout().Scale(overlayWidth-overlayPad*2, overlayHeight-overlayPad*2)
	g.overlayLabel.Base().Layout().SetOffset(overlayPad, overlayPad)
	g.overlayRoot.AddChild(g.overlayLabel.Base())
	g.refreshOverlay()
	g.overlayRoot.Base().Hide()
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
	title.SetFontFace(uiFontFace)
	title.SetFontSize(30)
	title.SetColor(matrix.ColorWhite())
	title.SetBGColor(matrix.ColorTransparent())
	title.SetJustify(rendering.FontJustifyCenter)
	title.SetBaseline(rendering.FontBaselineCenter)
	title.SetWrap(false)
	title.SetMaxWidth(520)
	title.Base().Layout().Scale(520, 64)
	title.Base().Layout().SetOffset(110, 96)
	g.menuRoot.AddChild(title.Base())

	start := g.createMenuButton("Start Benchmark", 170, 194, func() {
		logGame("start benchmark selected")
		g.startBenchmark()
	})
	g.menuRoot.AddChild(start.Base())

	quit := g.createMenuButton("Quit", 170, 260, func() {
		logGame("quit selected from main menu")
		g.host.Close()
	})
	g.menuRoot.AddChild(quit.Base())
}

func (g *Game) createMenuButton(text string, x, y float32, click func()) *ui.Button {
	const (
		menuButtonWidth  = 300
		menuButtonHeight = 48
	)
	tex, _ := g.host.TextureCache().Texture(assets.TextureSquare, rendering.TextureFilterNearest)
	button := g.uiManager.Add().ToButton()
	button.Init(tex, text)
	button.SetColor(matrix.ColorRGBAInt(230, 234, 241, 255))
	button.Base().Layout().Scale(menuButtonWidth, menuButtonHeight)
	button.Base().Layout().SetOffset(x, y)

	label := button.Label()
	label.SetText(text)
	label.SetFontFace(uiFontFace)
	label.SetFontSize(19)
	label.SetColor(matrix.ColorBlack())
	label.SetBGColor(matrix.ColorTransparent())
	label.SetMaxWidth(menuButtonWidth)
	label.Base().Layout().Scale(menuButtonWidth, menuButtonHeight)
	label.Base().Layout().SetOffset(0, 0)

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
	if err := g.loadAnimation(); err != nil {
		logGame("failed to load benchmark animation", "error", err)
	}
	if g.menuRoot != nil {
		g.host.DestroyEntity(g.menuRoot.Base().Entity())
		g.menuRoot = nil
	}
	g.overlayRoot.Base().Show()
	g.createWalls()
	g.refreshOverlay()
}

func (g *Game) loadAnimation() error {
	if len(g.animation) == len(benchmarkFrameKeys) {
		return nil
	}
	g.animation = g.animation[:0]
	for _, key := range benchmarkFrameKeys {
		tex, err := g.host.TextureCache().Texture(key, rendering.TextureFilterNearest)
		if err != nil {
			return err
		}
		g.animation = append(g.animation, tex)
	}
	logGame("loaded animated sprite frames", "frames", len(g.animation))
	return nil
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
	g.simulate2D(deltaTime)
	g.updateFPS(deltaTime)
}

func (g *Game) lateUpdate(deltaTime float64) {
	if !g.benchmarkOn {
		return
	}
	for i := range g.objects {
		t := &g.objects[i].sprite.Entity.Transform
		p := t.Position()
		if !matrix.Approx(p.Z(), 0) {
			p.SetZ(0)
			t.SetPosition(p)
		}
		t.SetRotation(matrix.Vec3Zero())
	}
}

func (g *Game) simulate2D(deltaTime float64) {
	if deltaTime <= 0 || len(g.objects) == 0 {
		return
	}

	halfSize := float64(spriteSize) * 0.5
	minX := -float64(windowWidth)*0.5 + halfSize
	maxX := float64(windowWidth)*0.5 - halfSize
	minY := -float64(windowHeight)*0.5 + halfSize
	maxY := float64(windowHeight)*0.5 - halfSize

	grid := make(map[[2]int][]int, len(g.objects))
	cellSize := float64(spriteSize)
	for i := range g.objects {
		obj := &g.objects[i]
		obj.x += obj.vx * deltaTime
		obj.y += obj.vy * deltaTime

		if obj.x < minX {
			obj.x = minX
			obj.vx = math.Abs(obj.vx)
		} else if obj.x > maxX {
			obj.x = maxX
			obj.vx = -math.Abs(obj.vx)
		}
		if obj.y < minY {
			obj.y = minY
			obj.vy = math.Abs(obj.vy)
		} else if obj.y > maxY {
			obj.y = maxY
			obj.vy = -math.Abs(obj.vy)
		}

		cell := [2]int{
			int(math.Floor((obj.x - minX) / cellSize)),
			int(math.Floor((obj.y - minY) / cellSize)),
		}
		grid[cell] = append(grid[cell], i)
	}

	for i := range g.objects {
		a := &g.objects[i]
		cx := int(math.Floor((a.x - minX) / cellSize))
		cy := int(math.Floor((a.y - minY) / cellSize))
		for gy := cy - 1; gy <= cy+1; gy++ {
			for gx := cx - 1; gx <= cx+1; gx++ {
				for _, j := range grid[[2]int{gx, gy}] {
					if j <= i {
						continue
					}
					g.resolveCollision(a, &g.objects[j])
				}
			}
		}
	}

	for i := range g.objects {
		obj := &g.objects[i]
		obj.frameTimer += deltaTime
		if len(g.animation) > 0 && obj.frameTimer >= 0.125 {
			obj.frameTimer = 0
			obj.frame = (obj.frame + 1) % len(g.animation)
			obj.sprite.SetTexture(g.animation[obj.frame])
		}
		if obj.flashTimer > 0 {
			obj.flashTimer = math.Max(0, obj.flashTimer-deltaTime)
			obj.sprite.SetColor(matrix.NewColor(1.8, 1.8, 1.8, 1))
		} else {
			obj.sprite.SetColor(obj.baseColor)
		}
		obj.sprite.SetPosition(float32(math.Round(obj.x)), float32(math.Round(obj.y)))
		obj.sprite.Entity.Transform.SetRotation(matrix.Vec3Zero())
	}
}

func (g *Game) resolveCollision(a, b *benchmarkObject) {
	dx := b.x - a.x
	dy := b.y - a.y
	overlapX := float64(spriteSize) - math.Abs(dx)
	overlapY := float64(spriteSize) - math.Abs(dy)
	if overlapX <= 0 || overlapY <= 0 {
		return
	}
	a.flashTimer = flashDuration
	b.flashTimer = flashDuration

	if overlapX < overlapY {
		sign := 1.0
		if dx < 0 {
			sign = -1
		}
		a.x -= sign * overlapX * 0.5
		b.x += sign * overlapX * 0.5
		a.vx, b.vx = b.vx, a.vx
		return
	}

	sign := 1.0
	if dy < 0 {
		sign = -1
	}
	a.y -= sign * overlapY * 0.5
	b.y += sign * overlapY * 0.5
	a.vy, b.vy = b.vy, a.vy
}

func (g *Game) enforceFixedResolution() {
	if g.host.Window.Width() != windowWidth || g.host.Window.Height() != windowHeight {
		logGame("window size changed", "width", g.host.Window.Width(), "height", g.host.Window.Height())
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
		g.resetBenchmark()
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

func (g *Game) resetBenchmark() {
	logGame("reloading benchmark", "oldObjects", len(g.objects), "targetObjects", defaultObjects)
	for i := range g.objects {
		g.host.DestroyEntity(&g.objects[i].sprite.Entity)
	}
	g.objects = g.objects[:0]
	g.targetCount = defaultObjects
	g.lastObjectLog = -1
	g.smoothedFPS = 0
	g.uiTimer = 0
	g.refreshOverlay()
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
	frame := 0
	baseColor := color
	if len(g.animation) > 0 {
		frame = g.rng.Intn(len(g.animation))
		baseColor = matrix.ColorWhite()
		s.InitFromTexture(float32(x), float32(y), spriteSize, spriteSize, g.host, g.animation[0], matrix.ColorWhite())
		s.SetUnBlended()
		s.SetUVs(0, matrix.NewVec4(0, 0, 1, 1))
		if frame != 0 {
			s.SetTexture(g.animation[frame])
		}
	} else {
		s.Init(float32(x), float32(y), spriteSize, spriteSize, g.host, assets.TextureSquare, color)
		s.SetUnBlended()
		s.SetUVs(0, matrix.NewVec4(0, 0, 1, 1))
	}

	vx := float64(randomRange(g.rng, -140, 140))
	vy := float64(randomRange(g.rng, -140, 140))
	if math.Abs(vx)+math.Abs(vy) < 80 {
		vx = 120
		vy = 80
	}

	return benchmarkObject{
		sprite:     s,
		baseColor:  baseColor,
		x:          float64(x),
		y:          float64(y),
		vx:         vx,
		vy:         vy,
		frame:      frame,
		frameTimer: g.rng.Float64() * 0.125,
	}
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

func copyCustomContent() error {
	if _, err := os.Stat(customContentPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return filepath.WalkDir(customContentPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(customContentPath, path)
		if err != nil {
			return err
		}
		outPath := filepath.Join(gameContentPath, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), os.ModePerm); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, data, os.ModePerm); err != nil {
			return err
		}
		return nil
	})
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
