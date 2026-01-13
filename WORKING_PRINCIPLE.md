# BallBattle 游戏工作原理详解

## 📋 目录
1. [整体架构](#整体架构)
2. [服务器工作原理](#服务器工作原理)
3. [客户端工作原理](#客户端工作原理)
4. [网络通信流程](#网络通信流程)
5. [帧同步机制](#帧同步机制)
6. [游戏逻辑处理](#游戏逻辑处理)

---

## 🏗️ 整体架构

```
┌─────────────────┐         UDP         ┌─────────────────┐
│                 │ ◄─────────────────► │                 │
│   客户端 Client  │    (输入 + 帧数据)   │   服务器 Server  │
│                 │                      │                 │
└─────────────────┘                      └─────────────────┘
      │                                          │
      │                                          │
      ▼                                          ▼
┌─────────────────┐                      ┌─────────────────┐
│   Ebiten 渲染   │                      │   GameLogic      │
│   (图形界面)    │                      │   (游戏逻辑)     │
└─────────────────┘                      └─────────────────┘
```

### 核心组件

1. **服务器端 (Server)**
   - `netcore.Server`: 网络层框架，处理 UDP 通信
   - `BallBattleLogic`: 游戏逻辑实现，实现 `GameLogic` 接口
   - `State`: 游戏状态管理（玩家、食物）

2. **客户端端 (Client)**
   - `Client`: 网络客户端，处理与服务器的通信
   - `Game`: Ebiten 游戏循环，处理渲染和输入
   - `GameState`: 本地游戏状态缓存

---

## 🖥️ 服务器工作原理

### 启动流程

```go
// cmd/server/main.go
1. 解析命令行参数（监听地址、tick率、食物数量、竞技场大小）
2. 创建游戏状态 (game.NewState)
3. 创建游戏逻辑 (game.NewBallBattleLogic)
4. 创建网络服务器 (netcore.NewServer)
5. 启动4个并发协程：
   - ListenLoop()          // 接收客户端数据
   - BroadcastLoop()       // 广播游戏帧
   - ReliableRetransmitLoop() // 可靠消息重传
   - CheckPlayerTimeout()   // 玩家超时检测
```

### 四个核心循环

#### 1. **ListenLoop() - 接收循环**
```go
功能：
- 持续监听 UDP 端口，接收客户端数据包
- 解析数据包类型：
  * 输入包 (InputPacket): 存储到 inputs map
  * 可靠消息: 处理 Ping/Pong 等
- 自动注册新玩家（当收到输入包时）
- 更新玩家最后活跃时间
- 输入限流（防止恶意 flooding）
```

**关键代码流程：**
```
收到 UDP 包
  ↓
解析 UDP 头部（序列号、ACK、ACK bits）
  ↓
判断数据包类型：
  ├─ 输入包 → 存储到 inputs[tick][playerID] = input
  └─ 可靠消息 → 处理 Ping/Pong 等
```

#### 2. **BroadcastLoop() - 广播循环**
```go
功能：
- 每 tick (默认 60Hz，即每 16.67ms) 执行一次
- 收集当前 tick 的所有玩家输入
- 调用游戏逻辑的 Tick() 方法更新游戏状态
- 生成游戏状态快照 (Snapshot)
- 广播给所有连接的玩家
```

**关键代码流程：**
```
定时器触发 (每 16.67ms)
  ↓
tick++
  ↓
收集 inputs[tick] (如果没有输入，填充 InputNone=0)
  ↓
调用 logic.Tick(tick, inputs) → 更新游戏状态
  ↓
调用 logic.Snapshot(tick) → 生成快照
  ↓
打包：FramePacket + Snapshot
  ↓
广播给所有玩家
```

#### 3. **ReliableRetransmitLoop() - 可靠重传循环**
```go
功能：
- 每 100ms 检查一次待重传的可靠消息
- 如果消息超过 200ms 未收到 ACK，则重传
- 确保重要消息（如 Ping/Pong）的可靠传输
```

#### 4. **CheckPlayerTimeout() - 超时检测循环**
```go
功能：
- 每 5 秒检查一次所有玩家
- 如果玩家超过 30 秒未活跃，则移除
- 调用 logic.OnLeave() 清理玩家数据
```

### 游戏逻辑接口实现

```go
// internal/game/logic.go

type BallBattleLogic struct {
    state *State
}

// OnJoin: 玩家加入时，在随机位置生成玩家
func (l *BallBattleLogic) OnJoin(pid uint16) {
    l.state.AddPlayer(pid)  // 创建玩家，随机位置，初始半径 1.2
}

// OnLeave: 玩家离开时，从状态中移除
func (l *BallBattleLogic) OnLeave(pid uint16) {
    l.state.RemovePlayer(pid)
}

// Tick: 每 tick 调用，应用所有输入
func (l *BallBattleLogic) Tick(tick uint32, inputs map[uint16]uint32) {
    for pid, input := range inputs {
        if input != 0 {  // InputNone=0 不需要处理
            l.state.ApplyInput(pid, input)
        }
    }
}

// Snapshot: 生成游戏状态快照（序列化为二进制）
func (l *BallBattleLogic) Snapshot(tick uint32) ([]byte, error) {
    // 序列化：玩家数量 + 玩家数据 + 食物数量 + 食物数据
}
```

### 游戏状态更新 (State.ApplyInput)

```go
// internal/game/state.go

func (s *State) ApplyInput(pid uint16, input uint32) {
    1. 获取或创建玩家
    2. 计算移动速度（根据玩家半径：越大越慢）
    3. 根据输入方向移动玩家：
       - InputLeft:  X -= speed
       - InputRight: X += speed
       - InputUp:    Y += speed
       - InputDown:  Y -= speed
    4. 限制在竞技场范围内
    5. 检测碰撞食物：
       - 如果碰撞，玩家半径增加，删除食物，生成新食物
}
```

---

## 💻 客户端工作原理

### 启动流程

```go
// cmd/client/main.go
1. 解析命令行参数（玩家ID、服务器地址、tick率）
2. 创建 UDP 连接
3. 创建 Client 实例（包含可靠传输组件）
4. 启动3个并发协程：
   - RecvLoop()              // 接收服务器数据
   - ReliableRetransmitLoop() // 可靠消息重传
   - InputLoop()             // 发送输入循环
5. 启动 Ebiten 游戏循环（渲染 + 输入处理）
```

### 三个核心循环

#### 1. **RecvLoop() - 接收循环**
```go
功能：
- 持续接收服务器 UDP 数据包
- 解析数据包类型：
  * 帧数据包 (FramePacket): 包含 tick 和所有玩家输入
  * 快照数据 (Snapshot): 包含完整的游戏状态
- 更新本地游戏状态 (GameState)
- 同步本地 tick 到服务器 tick
```

**关键代码流程：**
```
收到 UDP 包
  ↓
解析 UDP 头部（处理 ACK）
  ↓
解析帧数据包 (FramePacket)
  ↓
同步 localTick = max(localTick, serverTick)
  ↓
读取快照长度前缀
  ↓
解析玩家数据 → 更新 gameState.Players
  ↓
解析食物数据 → 更新 gameState.Foods
```

#### 2. **InputLoop() - 输入循环**
```go
功能：
- 每 tick (60Hz) 发送一次当前输入
- 使用 localTick + 1 作为预测 tick
- 发送输入包到服务器
```

**关键代码流程：**
```
定时器触发 (每 16.67ms)
  ↓
读取 currentInput（由 Ebiten Update() 更新）
  ↓
发送输入包：tick = localTick + 1
  ↓
localTick++
```

#### 3. **ReliableRetransmitLoop() - 可靠重传循环**
```go
功能：
- 与服务器类似，重传未确认的可靠消息
```

### Ebiten 游戏循环

#### Update() - 更新循环
```go
功能：
- 检测键盘输入（WASD 或方向键）
- 更新 currentInput
- 更新相机位置（跟随自己的玩家）
- 更新调试信息
```

**输入优先级：**
```
上下 > 左右
即：如果同时按下 W 和 A，优先处理 W（向上）
```

#### Draw() - 渲染循环
```go
功能：
- 绘制背景
- 绘制所有食物（绿色小圆）
- 绘制所有玩家（彩色大圆，自己的玩家有白色边框）
- 绘制 UI 信息（玩家数量、位置等）
- 绘制操作提示
```

**坐标转换：**
```go
世界坐标 → 屏幕坐标：
sx = screenW/2 + (wx - cameraX) * scale
sy = screenH/2 - (wy - cameraY) * scale  // Y轴翻转
```

---

## 📡 网络通信流程

### 数据包格式

#### 1. **UDP 头部**
```
[PacketSeq: uint16] [ACK: uint16] [ACKBits: uint32] [Payload: ...]
```

#### 2. **输入包 (InputPacket)**
```
[Tick: uint32] [PlayerID: uint16] [Input: uint32] [TS: int64]
```

#### 3. **帧数据包 (FramePacket)**
```
[Tick: uint32] [InputCount: uint8] 
  [PlayerID: uint16, Input: uint32] * InputCount
[SnapshotLength: uint16] [SnapshotData: ...]
```

#### 4. **快照数据 (Snapshot)**
```
[PlayerCount: uint8]
  [ID: uint16, X: float32, Y: float32, Radius: float32] * PlayerCount
[FoodCount: uint16]
  [ID: uint32, X: float32, Y: float32, Value: float32, Radius: float32] * FoodCount
```

### 通信时序图

```
客户端                          服务器
  │                               │
  │─── 输入包 (tick=1) ──────────►│
  │                               │ 存储输入
  │                               │
  │                               │ BroadcastLoop (tick=1)
  │                               │ ├─ 收集输入
  │                               │ ├─ Tick() 更新状态
  │                               │ ├─ Snapshot() 生成快照
  │                               │ └─ 广播帧数据
  │◄── 帧数据 (tick=1 + 快照) ────│
  │                               │
  │ 解析快照，更新本地状态         │
  │ 渲染画面                      │
  │                               │
  │─── 输入包 (tick=2) ──────────►│
  │                               │ ...
```

---

## ⚙️ 帧同步机制

### 核心原理

1. **服务器权威**
   - 服务器是游戏状态的唯一权威来源
   - 所有输入都在服务器上处理
   - 客户端只负责显示服务器状态

2. **固定 Tick 率**
   - 默认 60Hz（每秒 60 次更新）
   - 服务器每 16.67ms 执行一次 Tick
   - 客户端每 16.67ms 发送一次输入

3. **输入预测**
   - 客户端发送输入时使用 `localTick + 1`
   - 给服务器处理时间，避免输入延迟

4. **状态同步**
   - 服务器每 tick 广播完整快照
   - 客户端收到快照后直接覆盖本地状态
   - 简单但有效，适合小规模游戏

### Tick 同步

```
服务器 Tick:  0    1    2    3    4    5
              │    │    │    │    │    │
客户端输入:    └─►  └─►  └─►  └─►  └─►
              (tick+1预测)

服务器广播:         └─►  └─►  └─►  └─►  └─►
              (包含快照)
```

---

## 🎮 游戏逻辑处理

### 玩家移动

```go
速度计算：
speedFactor = 1.5 / (1.0 + radius)
if speedFactor < 0.4: speedFactor = 0.4
speed = 2.0 * speedFactor

规则：玩家越大，移动越慢（最小速度限制）
```

### 食物系统

```go
初始生成：
- 服务器启动时生成指定数量的食物（默认 120 个）
- 随机位置，随机 ID

碰撞检测：
- 使用圆形碰撞检测
- 距离² <= (玩家半径 + 食物半径)²

吃食物：
- 玩家半径 += 食物值 (0.15)
- 删除被吃的食物
- 立即生成新食物（保持总数不变）
```

### 竞技场边界

```go
范围：[-arenaHalf, arenaHalf] × [-arenaHalf, arenaHalf]
默认：[-100, 100] × [-100, 100]

玩家移动后会被限制在边界内：
X = clamp(X, -arenaHalf, arenaHalf)
Y = clamp(Y, -arenaHalf, arenaHalf)
```

---

## 🔄 完整游戏流程示例

### 场景：玩家 A 加入游戏并移动

```
1. 客户端启动
   ├─ 创建 UDP 连接
   ├─ 启动 RecvLoop、InputLoop、ReliableRetransmitLoop
   └─ 启动 Ebiten 游戏循环

2. 玩家按下 W 键
   ├─ Ebiten Update() 检测到按键
   ├─ 设置 currentInput = InputUp
   └─ InputLoop() 发送输入包 (tick=1, input=InputUp)

3. 服务器接收输入
   ├─ ListenLoop() 收到输入包
   ├─ 自动注册玩家（如果未注册）
   ├─ 存储输入：inputs[1][playerA] = InputUp
   └─ 更新玩家最后活跃时间

4. 服务器 Tick 处理
   ├─ BroadcastLoop() 触发 (tick=1)
   ├─ 收集所有玩家输入（包括玩家 A）
   ├─ 调用 logic.Tick(1, inputs)
   │  └─ state.ApplyInput(playerA, InputUp)
   │     ├─ 计算速度
   │     ├─ 移动玩家：Y += speed
   │     ├─ 限制边界
   │     └─ 检测食物碰撞
   ├─ 调用 logic.Snapshot(1)
   │  └─ 序列化所有玩家和食物数据
   └─ 广播帧数据包（包含快照）

5. 客户端接收并渲染
   ├─ RecvLoop() 收到帧数据包
   ├─ 解析快照，更新 gameState
   ├─ Ebiten Draw() 渲染画面
   │  ├─ 绘制食物
   │  ├─ 绘制玩家（包括玩家 A）
   │  └─ 更新相机位置（跟随玩家 A）
   └─ 玩家看到自己向上移动
```

---

## 🎯 关键设计特点

1. **解耦设计**
   - 网络层 (`netcore`) 与游戏逻辑 (`GameLogic`) 完全分离
   - 可以轻松替换不同的游戏逻辑实现

2. **可靠传输**
   - UDP + 自定义可靠传输层
   - 支持消息重传和 ACK 确认

3. **性能优化**
   - O(1) 地址查找
   - 内存池复用
   - 输入限流

4. **简单有效**
   - 状态快照同步，实现简单
   - 适合小规模实时多人游戏

---

## 📝 总结

BallBattle 是一个基于 **帧同步** 的实时多人游戏：

- **服务器**：每 tick 收集输入 → 更新状态 → 广播快照
- **客户端**：发送输入 → 接收快照 → 渲染画面
- **同步**：服务器权威，客户端显示服务器状态

这种设计简单可靠，适合快速原型开发和小规模游戏。



