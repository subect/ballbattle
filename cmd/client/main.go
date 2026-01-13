package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"gameframework/pkg/proto"
	"gameframework/pkg/reliable"
	"image/color"
	"net"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// 输入常量
const (
	InputNone  = 0
	InputLeft  = 1
	InputRight = 2
	InputUp    = 3
	InputDown  = 4
)

// 玩家数据
type Player struct {
	ID     uint16
	X      float32
	Y      float32
	Radius float32
}

// 食物数据
type Food struct {
	ID     uint32
	X      float32
	Y      float32
	Value  float32
	Radius float32
}

// 游戏状态
type GameState struct {
	mu      sync.RWMutex
	Players map[uint16]*Player
	Foods   map[uint32]*Food
	MyID    uint16
}

func NewGameState() *GameState {
	return &GameState{
		Players: make(map[uint16]*Player),
		Foods:   make(map[uint32]*Food),
	}
}

// 客户端
type Client struct {
	id         uint16
	conn       *net.UDPConn
	serverAddr *net.UDPAddr

	rxReliable *reliable.ReliableReceiver
	txReliable *reliable.ReliableSender

	gameState *GameState
	joined    bool

	// 输入相关
	currentInput uint32
	inputMu      sync.Mutex
	localTick    uint32
}

func NewClient(id uint16, serverAddr string) (*Client, error) {
	addr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		id:           id,
		conn:         conn,
		serverAddr:   addr,
		rxReliable:   reliable.NewReliableReceiver(),
		txReliable:   reliable.NewReliableSender(),
		gameState:    NewGameState(),
		currentInput: InputNone,
		joined:       true, // 直接允许发送输入，服务端收到输入时注册玩家
	}
	c.gameState.MyID = id

	return c, nil
}

// 发送输入
func (c *Client) SendInput(tick uint32, input uint32) error {
	p := &proto.InputPacket{
		Tick:     tick,
		PlayerID: c.id,
		Input:    input,
		TS:       time.Now().UnixNano(),
	}

	buf := &bytes.Buffer{}
	proto.WriteInputPacket(buf, p)

	ack, ackbits := c.rxReliable.BuildAckAndBits()
	packetSeq := c.txReliable.NextPacketSeq()

	headerBuf := &bytes.Buffer{}
	proto.WriteUDPHeader(headerBuf, packetSeq, ack, ackbits)
	headerBuf.Write(buf.Bytes())

	_, err := c.conn.WriteToUDP(headerBuf.Bytes(), c.serverAddr)
	return err
}

// 接收循环
func (c *Client) RecvLoop() {
	buf := make([]byte, 4096)
	fmt.Println("📡 开始接收循环...")
	for {
		n, raddr, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Printf("⚠ 接收错误: %v\n", err)
			continue
		}

		if raddr.String() != c.serverAddr.String() {
			// 忽略非服务器来源的数据包
			continue
		}

		_, ack, ackBits, payload, err := proto.ReadUDPHeader(buf[:n])
		if err != nil {
			fmt.Printf("⚠ UDP头部解析失败: %v\n", err)
			continue
		}

		fmt.Printf("📥 收到数据包: len=%d, payload len=%d\n", n, len(payload))

		// 处理 ACK
		if c.txReliable != nil {
			c.txReliable.ProcessAckFromRemote(ack, ackBits)
		}

		// 先尝试解析帧数据（因为帧数据更常见，且不是可靠消息）
		tick, _, err := proto.ReadFramePacket(payload)
		if err == nil {
			// 同步本地 tick 到服务器 tick（重要：确保输入发送的 tick 与服务器同步）
			if tick > c.localTick {
				c.localTick = tick
			}
			// fmt.Printf("✓ 成功解析帧数据包: tick=%d, inputs=%d\n", tick, len(inputs))
			// 读取快照数据
			r := bytes.NewReader(payload)
			// 跳过已读的帧数据
			var tempTick uint32
			var tempCount uint8
			binary.Read(r, binary.LittleEndian, &tempTick)
			binary.Read(r, binary.LittleEndian, &tempCount)
			for i := 0; i < int(tempCount); i++ {
				var pid uint16
				var in uint32
				binary.Read(r, binary.LittleEndian, &pid)
				binary.Read(r, binary.LittleEndian, &in)
			}

			// 读取快照长度前缀（uint16）
			var snapLen uint16
			if err := binary.Read(r, binary.LittleEndian, &snapLen); err != nil {
				// 没有快照数据，跳过
				fmt.Printf("⚠ 没有快照数据 (err: %v)\n", err)
				continue
			}
			if snapLen == 0 {
				fmt.Printf("⚠ 快照长度为0\n")
				continue
			}
			fmt.Printf("📦 快照长度: %d bytes\n", snapLen)
			c.joined = true

			// 读取玩家数据
			var playerCount uint8
			if err := binary.Read(r, binary.LittleEndian, &playerCount); err == nil {
				c.gameState.mu.Lock()
				fmt.Printf("📦 解析到 %d 个玩家\n", playerCount)
				for i := 0; i < int(playerCount); i++ {
					var p Player
					binary.Read(r, binary.LittleEndian, &p.ID)
					binary.Read(r, binary.LittleEndian, &p.X)
					binary.Read(r, binary.LittleEndian, &p.Y)
					binary.Read(r, binary.LittleEndian, &p.Radius)
					c.gameState.Players[p.ID] = &p
					if p.ID == c.gameState.MyID {
						fmt.Printf("✓ 收到我的玩家数据: ID=%d, pos=(%.1f, %.1f), radius=%.2f\n",
							p.ID, p.X, p.Y, p.Radius)
					} else {
						fmt.Printf("  - 玩家 %d: pos=(%.1f, %.1f), radius=%.2f\n",
							p.ID, p.X, p.Y, p.Radius)
					}
				}

				// 读取食物数据
				var foodCount uint16
				if err := binary.Read(r, binary.LittleEndian, &foodCount); err == nil {
					c.gameState.Foods = make(map[uint32]*Food)
					fmt.Printf("📦 解析到 %d 个食物\n", foodCount)
					for i := 0; i < int(foodCount); i++ {
						var f Food
						binary.Read(r, binary.LittleEndian, &f.ID)
						binary.Read(r, binary.LittleEndian, &f.X)
						binary.Read(r, binary.LittleEndian, &f.Y)
						binary.Read(r, binary.LittleEndian, &f.Value)
						binary.Read(r, binary.LittleEndian, &f.Radius)
						c.gameState.Foods[f.ID] = &f
					}
					if foodCount > 0 && len(c.gameState.Players) > 0 {
						fmt.Printf("✓ 收到完整游戏数据: %d 玩家, %d 食物\n",
							len(c.gameState.Players), int(foodCount))
					}
				} else {
					fmt.Printf("⚠ 读取食物数据失败: %v\n", err)
				}
				c.gameState.mu.Unlock()
			} else {
				fmt.Printf("⚠ 读取玩家数据失败: %v\n", err)
			}
		} else if rseq, inner, err2 := proto.UnpackReliableEnvelope(payload); err2 == nil {
			// 仅处理 Ping/Pong 等通用可靠消息
			c.rxReliable.MarkReceived(rseq)
			if !c.rxReliable.AlreadyProcessed(rseq) {
				c.rxReliable.MarkProcessed(rseq)
				if len(inner) > 0 && inner[0] == proto.MsgPong {
					fmt.Printf("收到 PONG\n")
				}
			}
		} else if len(payload) > 4 {
			fmt.Printf("⚠ 未知数据包: 帧解析err=%v, 可靠解析err=%v, payload len=%d, first 4 bytes: %x\n",
				err, err2, len(payload), payload[:4])
		}
	}
}

// 可靠重传循环
func (c *Client) ReliableRetransmitLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		pend := c.txReliable.GetPendingOlderThan(200)
		for _, pm := range pend {
			ack, ackbits := c.rxReliable.BuildAckAndBits()
			packetSeq := c.txReliable.NextPacketSeq()
			buf := &bytes.Buffer{}
			proto.WriteUDPHeader(buf, packetSeq, ack, ackbits)
			proto.PackReliableEnvelope(buf, pm.Seq, pm.Payload)
			c.conn.WriteToUDP(buf.Bytes(), c.serverAddr)
			c.txReliable.UpdatePendingSent(pm.Seq)
		}
	}
}

// 输入循环
func (c *Client) InputLoop(tickHz int) {
	ticker := time.NewTicker(time.Duration(1000/tickHz) * time.Millisecond)
	fmt.Println("⌨️  输入循环已启动")
	for range ticker.C {
		c.inputMu.Lock()
		input := c.currentInput
		c.inputMu.Unlock()

		// 发送输入时，使用当前 localTick + 1（预测下一帧，给服务器处理时间）
		// 注意：localTick 会在收到服务器帧时同步更新
		sendTick := c.localTick + 1
		if err := c.SendInput(sendTick, input); err != nil {
			fmt.Printf("⚠ 发送输入失败: %v\n", err)
		} else if input != InputNone {
			fmt.Printf("⌨️  发送输入: tick=%d, input=%d (%s)\n", sendTick, input,
				map[uint32]string{InputLeft: "左", InputRight: "右", InputUp: "上", InputDown: "下"}[input])
		}

		c.localTick++
	}
}

// 游戏结构（实现 ebiten.Game 接口）
type Game struct {
	client   *Client
	screenW  int
	screenH  int
	cameraX  float32
	cameraY  float32
	scale    float32
	debugMsg string
}

func NewGame(client *Client) *Game {
	return &Game{
		client:   client,
		screenW:  800,
		screenH:  600,
		scale:    3.0, // 增大缩放，让物体更明显
		debugMsg: "等待连接...",
	}
}

func (g *Game) Update() error {
	// 处理键盘输入（优先级：上下 > 左右）
	var input uint32 = InputNone

	// 检查所有可能的按键
	upPressed := ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW)
	downPressed := ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS)
	leftPressed := ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA)
	rightPressed := ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD)

	// 检查是否有任何按键被按下
	anyKeyPressed := upPressed || downPressed || leftPressed || rightPressed

	if upPressed {
		input = InputUp
	} else if downPressed {
		input = InputDown
	} else if leftPressed {
		input = InputLeft
	} else if rightPressed {
		input = InputRight
	}

	g.client.inputMu.Lock()
	oldInput := g.client.currentInput
	g.client.currentInput = input
	g.client.inputMu.Unlock()

	// 调试：打印按键状态（只在有按键按下时打印，避免刷屏）
	if anyKeyPressed && input != oldInput {
		fmt.Printf("🎮 按键检测: W=%v A=%v S=%v D=%v → input=%d (old=%d)\n",
			upPressed, leftPressed, downPressed, rightPressed, input, oldInput)
	}

	// 更新相机位置（跟随我的玩家）
	g.client.gameState.mu.RLock()
	myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
	if myPlayer != nil {
		g.cameraX = myPlayer.X
		g.cameraY = myPlayer.Y
		g.debugMsg = fmt.Sprintf("已连接 | 玩家:%d 食物:%d",
			len(g.client.gameState.Players), len(g.client.gameState.Foods))
	} else {
		if g.client.joined {
			g.debugMsg = "已加入，等待玩家数据..."
		} else {
			g.debugMsg = "等待加入游戏..."
		}
	}
	g.client.gameState.mu.RUnlock()

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{20, 20, 30, 255}) // 深色背景

	g.client.gameState.mu.RLock()
	defer g.client.gameState.mu.RUnlock()

	// 世界坐标到屏幕坐标的转换
	worldToScreen := func(wx, wy float32) (sx, sy float32) {
		sx = float32(g.screenW)/2 + (wx-g.cameraX)*g.scale
		sy = float32(g.screenH)/2 - (wy-g.cameraY)*g.scale // Y轴翻转（世界坐标Y向上，屏幕Y向下）
		return
	}

	// 绘制食物
	for _, f := range g.client.gameState.Foods {
		sx, sy := worldToScreen(f.X, f.Y)
		radius := f.Radius * g.scale

		// 只绘制在屏幕范围内的食物
		if sx >= -50 && sx <= float32(g.screenW)+50 && sy >= -50 && sy <= float32(g.screenH)+50 {
			// 绘制食物（绿色小圆）
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), radius, color.RGBA{100, 200, 100, 255}, true)
		}
	}

	// 绘制玩家
	for _, p := range g.client.gameState.Players {
		sx, sy := worldToScreen(p.X, p.Y)
		radius := p.Radius * g.scale

		// 只绘制在屏幕范围内的玩家
		if sx >= -100 && sx <= float32(g.screenW)+100 && sy >= -100 && sy <= float32(g.screenH)+100 {
			// 所有玩家都根据 ID 使用相同的颜色算法，确保在不同客户端看到相同颜色
			colors := []color.RGBA{
				{100, 150, 255, 255}, // 蓝（ID 0）
				{255, 100, 100, 255}, // 红（ID 1）
				{255, 200, 100, 255}, // 橙（ID 2）
				{200, 100, 255, 255}, // 紫（ID 3）
				{100, 255, 200, 255}, // 青（ID 4）
				{255, 100, 200, 255}, // 粉（ID 5）
				{200, 255, 100, 255}, // 黄绿（ID 6）
				{255, 255, 100, 255}, // 黄（ID 7）
			}
			colorIdx := int(p.ID) % len(colors)
			playerColor := colors[colorIdx]

			// 绘制玩家球
			vector.DrawFilledCircle(screen, float32(sx), float32(sy), radius, playerColor, true)

			// 如果是自己的玩家，添加白色边框以区分
			if p.ID == g.client.gameState.MyID {
				vector.StrokeCircle(screen, float32(sx), float32(sy), radius, 2, color.RGBA{255, 255, 255, 255}, true)
			}
		}
	}

	// 绘制 UI 信息
	myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
	if myPlayer != nil {
		info := fmt.Sprintf("%s\nID: %d | 位置: (%.1f, %.1f) | 半径: %.2f | 相机: (%.1f, %.1f)",
			g.debugMsg,
			myPlayer.ID, myPlayer.X, myPlayer.Y, myPlayer.Radius,
			g.cameraX, g.cameraY)
		ebitenutil.DebugPrint(screen, info)
	} else {
		ebitenutil.DebugPrint(screen, g.debugMsg)
	}

	// 绘制操作提示
	controls := "方向键或 WASD 移动"
	ebitenutil.DebugPrintAt(screen, controls, 0, g.screenH-20)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.screenW, g.screenH
}

func main() {
	var playerID int
	var serverAddr string
	var tickHz int

	flag.IntVar(&playerID, "id", 1, "Player ID")
	flag.StringVar(&serverAddr, "server", "localhost:30000", "Server address")
	flag.IntVar(&tickHz, "hz", 60, "Tick rate")
	flag.Parse()

	client, err := NewClient(uint16(playerID), serverAddr)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// 启动网络循环
	go client.RecvLoop()
	go client.ReliableRetransmitLoop()
	go client.InputLoop(tickHz)

	fmt.Printf("Connecting to server %s as player %d...\n", serverAddr, playerID)
	fmt.Println("Use arrow keys or WASD to move")
	fmt.Println("💡 提示：请确保游戏窗口获得焦点（点击窗口），然后按 WASD 或方向键")

	// 创建游戏并运行
	game := NewGame(client)
	ebiten.SetWindowSize(800, 600)
	ebiten.SetWindowTitle("球球大作战 - Ball Battle")
	ebiten.SetWindowResizable(true)

	if err := ebiten.RunGame(game); err != nil {
		fmt.Printf("Game error: %v\n", err)
	}
}
