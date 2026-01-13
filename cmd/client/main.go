package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image/color"
	"math"
	"sync"
	"time"

	"ballbattle/internal/game"
	"gameframework/pkg/client"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	screenWidth  = 800
	screenHeight = 600
)

// Game wraps Ebiten game logic and network client
type Game struct {
	client   *client.Client
	playerID uint16

	// Render state (thread-safe)
	stateMu    sync.RWMutex
	players    map[uint16]*game.Player
	foods      map[uint32]*game.Food
	arenaHalf  float32
	serverTick uint32

	// Input tracking
	currentInput uint32
	lastSentTick uint32

	// Network stats
	rtt       time.Duration
	lossRate  float64
	statsTick int
}

func NewGame(c *client.Client, playerID uint16, arenaHalf float32) *Game {
	return &Game{
		client:    c,
		playerID:  playerID,
		players:   make(map[uint16]*game.Player),
		foods:     make(map[uint32]*game.Food),
		arenaHalf: arenaHalf,
	}
}

func (g *Game) Update() error {
	// Process keyboard input
	g.currentInput = 0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		g.currentInput = game.InputLeft
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		g.currentInput = game.InputRight
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		g.currentInput = game.InputUp
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		g.currentInput = game.InputDown
	}

	// Send input if changed or periodically
	if g.currentInput != 0 || g.serverTick != g.lastSentTick {
		tick := g.serverTick
		if tick == 0 {
			tick = 1 // start from 1
		}
		if err := g.client.SendInput(tick, g.currentInput); err == nil {
			g.lastSentTick = tick
		}
	}

	// Process incoming frame updates
	select {
	case fu := <-g.client.FrameUpdates():
		g.stateMu.Lock()
		g.serverTick = fu.Tick

		// Parse snapshot if available
		if len(fu.Snap) > 0 {
			g.parseSnapshot(fu.Snap)
		}
		g.stateMu.Unlock()
	default:
		// No new frame, continue rendering
	}

	// Update network stats periodically
	g.statsTick++
	if g.statsTick%60 == 0 {
		g.rtt = g.client.RTT()
		g.lossRate = g.client.LossRate()
		g.statsTick = 0 // Reset to avoid overflow
	}

	return nil
}

func (g *Game) parseSnapshot(data []byte) {
	r := bytes.NewReader(data)

	// Read players
	var playerCount uint8
	if err := binary.Read(r, binary.LittleEndian, &playerCount); err != nil {
		return
	}

	g.players = make(map[uint16]*game.Player, playerCount)
	for i := 0; i < int(playerCount); i++ {
		var pid uint16
		var x, y, radius float32
		if err := binary.Read(r, binary.LittleEndian, &pid); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &radius); err != nil {
			return
		}
		g.players[pid] = &game.Player{
			ID:     pid,
			X:      x,
			Y:      y,
			Radius: radius,
		}
	}

	// Read foods
	var foodCount uint16
	if err := binary.Read(r, binary.LittleEndian, &foodCount); err != nil {
		return
	}

	g.foods = make(map[uint32]*game.Food, foodCount)
	for i := 0; i < int(foodCount); i++ {
		var fid uint32
		var x, y, value, radius float32
		if err := binary.Read(r, binary.LittleEndian, &fid); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &x); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &y); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &value); err != nil {
			return
		}
		if err := binary.Read(r, binary.LittleEndian, &radius); err != nil {
			return
		}
		g.foods[fid] = &game.Food{
			ID:     fid,
			X:      x,
			Y:      y,
			Value:  value,
			Radius: radius,
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255}) // Dark background

	g.stateMu.RLock()
	players := make(map[uint16]*game.Player)
	foods := make(map[uint32]*game.Food)
	for k, v := range g.players {
		players[k] = v
	}
	for k, v := range g.foods {
		foods[k] = v
	}
	arenaHalf := g.arenaHalf
	g.stateMu.RUnlock()

	// Calculate viewport transform (world to screen)
	// World coordinates: [-arenaHalf, arenaHalf] for both X and Y
	// Screen coordinates: [0, screenWidth] x [0, screenHeight]
	scaleX := float32(screenWidth) / (2 * arenaHalf)
	scaleY := float32(screenHeight) / (2 * arenaHalf)
	offsetX := float32(screenWidth) / 2
	offsetY := float32(screenHeight) / 2

	// Draw arena border
	borderColor := color.RGBA{100, 100, 120, 255}
	vector.StrokeLine(screen, 0, 0, float32(screenWidth), 0, 2, borderColor, false)
	vector.StrokeLine(screen, float32(screenWidth), 0, float32(screenWidth), float32(screenHeight), 2, borderColor, false)
	vector.StrokeLine(screen, float32(screenWidth), float32(screenHeight), 0, float32(screenHeight), 2, borderColor, false)
	vector.StrokeLine(screen, 0, float32(screenHeight), 0, 0, 2, borderColor, false)

	// Draw foods
	for _, f := range foods {
		sx := f.X*scaleX + offsetX
		sy := -f.Y*scaleY + offsetY // Flip Y axis (world Y+ is up, screen Y+ is down)
		radius := f.Radius * scaleX

		// Food color based on value
		foodColor := color.RGBA{100, 200, 100, 255}
		vector.DrawFilledCircle(screen, sx, sy, radius, foodColor, true)
	}

	// Draw players
	for pid, p := range players {
		sx := p.X*scaleX + offsetX
		sy := -p.Y*scaleY + offsetY
		radius := p.Radius * scaleX

		// Player color: highlight self, different colors for others
		var playerColor color.RGBA
		if pid == g.playerID {
			playerColor = color.RGBA{100, 150, 255, 255} // Blue for self
		} else {
			// Different colors for other players
			hue := float32(pid) * 0.618 // Golden ratio for color distribution
			playerColor = hsvToRGB(hue, 0.8, 0.9)
		}

		vector.DrawFilledCircle(screen, sx, sy, radius, playerColor, true)

		// Draw player ID label
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%d", pid), int(sx-5), int(sy-8))
	}

	// Draw UI overlay
	g.drawUI(screen)
}

func (g *Game) drawUI(screen *ebiten.Image) {
	// Network stats
	statsText := fmt.Sprintf("Tick: %d | RTT: %v | Loss: %.1f%%",
		g.serverTick, g.rtt.Round(time.Millisecond), g.lossRate*100)
	ebitenutil.DebugPrintAt(screen, statsText, 10, 10)

	// Instructions
	instructions := "WASD or Arrow Keys to move"
	ebitenutil.DebugPrintAt(screen, instructions, 10, screenHeight-20)

	// Player count
	g.stateMu.RLock()
	playerCount := len(g.players)
	foodCount := len(g.foods)
	g.stateMu.RUnlock()

	countText := fmt.Sprintf("Players: %d | Foods: %d", playerCount, foodCount)
	ebitenutil.DebugPrintAt(screen, countText, 10, 30)
}

func hsvToRGB(h, s, v float32) color.RGBA {
	h = h - float32(math.Floor(float64(h/360)))*360
	c := v * s
	x := c * (1 - float32(math.Abs(float64(math.Mod(float64(h/60), 2)-1))))
	m := v - c

	var r, g, b float32
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8((r + m) * 255),
		G: uint8((g + m) * 255),
		B: uint8((b + m) * 255),
		A: 255,
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	var (
		serverAddr = flag.String("server", "localhost:30000", "Server address (host:port)")
		playerID   = flag.Int("id", 1, "Player ID")
		tickHz     = flag.Int("hz", 60, "Tick rate (Hz)")
		arenaSize  = flag.Float64("size", 100, "Arena half-size (world units)")
	)
	flag.Parse()

	// Create client
	cfg := client.ClientConfig{
		ServerAddr: *serverAddr,
		PlayerID:   uint16(*playerID),
		TickHz:     *tickHz,
	}

	c, err := client.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "Failed to create client: %v\n", err)
		return
	}

	if err := c.Start(); err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "Failed to start client: %v\n", err)
		return
	}
	defer c.Close()

	fmt.Printf("Connecting to %s (PlayerID=%d, TickHz=%d)...\n", *serverAddr, *playerID, *tickHz)
	fmt.Println("Window will open shortly. Use WASD or Arrow Keys to move.")

	// Create game
	game := NewGame(c, uint16(*playerID), float32(*arenaSize))

	// Configure Ebiten
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("BallBattle - Player " + fmt.Sprintf("%d", *playerID))
	ebiten.SetWindowResizable(true)

	// Run game loop
	if err := ebiten.RunGame(game); err != nil {
		fmt.Fprintf(flag.CommandLine.Output(), "Game error: %v\n", err)
	}
}
