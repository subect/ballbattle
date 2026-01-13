# 底层网络框架修改总结

## 修改的文件

主要修改了 `gameframework/pkg/netcore/server.go` 文件，这是一个通用的网络层框架。

## 核心改动

### 1. **引入 GameLogic 接口（关键改动）** ✅

这是最重要的改动，使框架可以支持任意游戏逻辑：

```go
type GameLogic interface {
    OnJoin(pid uint16)                    // 玩家加入时回调
    OnLeave(pid uint16)                   // 玩家离开时回调
    ApplyInput(pid uint16, input uint32)  // 收到输入时回调（可选）
    Tick(tick uint32, inputs map[uint16]uint32)  // 每 tick 调用，应用所有输入
    Snapshot(tick uint32) ([]byte, error) // 返回游戏状态快照
}
```

**好处：**
- 网络层与游戏逻辑完全解耦
- 可以轻松替换不同的游戏逻辑实现
- 框架本身不包含任何游戏特定的代码

### 2. **性能优化**

#### 2.1 O(1) 查找优化
- 添加 `playersByAddr` map，实现 O(1) 地址查找
- 添加 `maxReceivedTick` 变量，避免每次遍历 inputs map

#### 2.2 内存池优化
- 使用 `sync.Pool` 复用 `bytes.Buffer`，减少内存分配

#### 2.3 输入限流
- 添加 `inputCount` 和 `inputWindow`，防止恶意输入 flooding

### 3. **玩家超时检测**

- 添加 `lastActive` 字段跟踪玩家最后活跃时间
- 添加 `CheckPlayerTimeout()` 方法，定期清理不活跃玩家

### 4. **输入处理优化**

- 修改 `BroadcastLoop`，确保即使没有新输入也发送帧（避免玩家超时）
- 使用 `InputNone (0)` 填充缺失输入，而不是使用 `lastInput`（避免玩家持续移动）

### 5. **调试日志**

- 添加输入接收、应用等关键步骤的调试日志
- 便于排查问题

## 是否可以应用到其他游戏？

**完全可以！** ✅

### 为什么可以复用？

1. **接口化设计**
   - `GameLogic` 接口完全抽象了游戏逻辑
   - 网络层不依赖任何具体的游戏实现

2. **通用网络协议**
   - 使用标准的 UDP + 可靠消息机制
   - 输入格式：`Tick + PlayerID + Input`
   - 帧格式：`Tick + Inputs + Snapshot`

3. **灵活的序列化**
   - `Snapshot()` 方法返回 `[]byte`，游戏可以自由定义序列化格式
   - 支持任意复杂度的游戏状态

### 如何应用到其他游戏？

#### 步骤 1：实现 GameLogic 接口

```go
type MyGameLogic struct {
    state *MyGameState
}

func (l *MyGameLogic) OnJoin(pid uint16) {
    // 初始化玩家
}

func (l *MyGameLogic) OnLeave(pid uint16) {
    // 清理玩家
}

func (l *MyGameLogic) ApplyInput(pid uint16, input uint32) {
    // 可选：即时处理输入
}

func (l *MyGameLogic) Tick(tick uint32, inputs map[uint16]uint32) {
    // 应用所有输入，更新游戏状态
    for pid, input := range inputs {
        l.state.ApplyInput(pid, input)
    }
}

func (l *MyGameLogic) Snapshot(tick uint32) ([]byte, error) {
    // 序列化游戏状态
    return l.state.Serialize(), nil
}
```

#### 步骤 2：创建服务器

```go
logic := NewMyGameLogic(state)
server, err := netcore.NewServer(":30000", 60, logic)
```

#### 步骤 3：客户端实现

客户端需要：
1. 发送输入：使用 `proto.WriteInputPacket`
2. 接收帧：解析 `proto.ReadFramePacket` 和快照数据
3. 渲染：根据快照数据更新显示

### 适用游戏类型

✅ **适合的游戏：**
- 实时多人游戏（RTS、MOBA、动作游戏）
- 需要帧同步的游戏
- 需要低延迟的游戏

❌ **不太适合的游戏：**
- 回合制游戏（可以用，但没必要）
- 纯服务器权威的游戏（不需要客户端预测）

### 示例：不同类型的游戏

#### 示例 1：2D 平台跳跃游戏
```go
// 输入：方向键 + 跳跃
const (
    InputLeft = 1
    InputRight = 2
    InputJump = 3
)

// 状态：位置、速度、是否在地面
type Player struct {
    X, Y, VX, VY float32
    OnGround bool
}
```

#### 示例 2：RTS 游戏
```go
// 输入：选择单位、移动命令
type Input struct {
    SelectUnit uint32
    MoveToX, MoveToY float32
}

// 状态：单位位置、生命值等
type Unit struct {
    ID uint32
    X, Y float32
    HP int
}
```

#### 示例 3：射击游戏
```go
// 输入：移动 + 射击方向
type Input struct {
    MoveX, MoveY float32
    AimAngle float32
    Shoot bool
}

// 状态：位置、角度、子弹等
type Player struct {
    X, Y, Angle float32
    Bullets []Bullet
}
```

## 总结

**修改的框架文件：**
- `gameframework/pkg/netcore/server.go` - 核心网络层

**关键特性：**
- ✅ 完全解耦的游戏逻辑接口
- ✅ 高性能（O(1) 查找、内存池）
- ✅ 玩家超时检测
- ✅ 输入限流
- ✅ 通用且可扩展

**可复用性：**
- ✅ 可以应用到任何需要帧同步的实时多人游戏
- ✅ 只需要实现 `GameLogic` 接口
- ✅ 网络层代码无需修改





