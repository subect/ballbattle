# 玩家如何定义自己的逻辑

## 📋 概述

在 BallBattle 游戏中，玩家可以通过修改客户端的**输入生成逻辑**来定义自己的行为。虽然服务器端处理游戏逻辑，但玩家可以在客户端实现自己的决策算法（如 AI、策略等）。

## 🎯 核心原理

### 当前架构

```
客户端 Update() 函数
    ↓
生成输入 (InputLeft/Right/Up/Down/None)
    ↓
发送到服务器
    ↓
服务器应用输入，更新游戏状态
    ↓
广播状态快照回客户端
```

### 关键代码位置

**文件：** `cmd/client/main.go`

**关键函数：** `Game.Update()` (第 316-368 行)

当前实现是读取键盘输入，你可以替换为任何自定义逻辑。

## 🔧 如何自定义玩家逻辑

### 方法 1：修改 Update() 函数

在 `cmd/client/main.go` 的 `Update()` 函数中，替换键盘检测部分：

```go
func (g *Game) Update() error {
    var input uint32 = InputNone
    
    // ===== 替换这部分代码 =====
    // 原来的键盘输入代码：
    // upPressed := ebiten.IsKeyPressed(ebiten.KeyW)
    // ...
    
    // 改为你的自定义逻辑：
    input = g.MyCustomLogic()
    // ============================
    
    g.client.inputMu.Lock()
    g.client.currentInput = input
    g.client.inputMu.Unlock()
    
    // ... 其余代码保持不变
    return nil
}
```

### 方法 2：实现自定义决策函数

添加一个决策函数，基于游戏状态生成输入：

```go
// 示例：简单的 AI 逻辑
func (g *Game) MyCustomLogic() uint32 {
    g.client.gameState.mu.RLock()
    defer g.client.gameState.mu.RUnlock()
    
    myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
    if myPlayer == nil {
        return InputNone
    }
    
    // 找到最近的食物
    var nearestFood *Food
    var minDist float32 = 1000.0
    
    for _, food := range g.client.gameState.Foods {
        dx := food.X - myPlayer.X
        dy := food.Y - myPlayer.Y
        dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
        if dist < minDist {
            minDist = dist
            nearestFood = food
        }
    }
    
    if nearestFood == nil {
        return InputNone
    }
    
    // 朝最近的食物移动
    dx := nearestFood.X - myPlayer.X
    dy := nearestFood.Y - myPlayer.Y
    
    if math.Abs(float64(dx)) > math.Abs(float64(dy)) {
        if dx > 0 {
            return InputRight
        } else {
            return InputLeft
        }
    } else {
        if dy > 0 {
            return InputUp
        } else {
            return InputDown
        }
    }
}
```

## 💡 示例：不同类型的自定义逻辑

### 示例 1：简单 AI - 追逐最近的食物

```go
func (g *Game) ChaseNearestFood() uint32 {
    g.client.gameState.mu.RLock()
    defer g.client.gameState.mu.RUnlock()
    
    myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
    if myPlayer == nil || len(g.client.gameState.Foods) == 0 {
        return InputNone
    }
    
    // 找到最近的食物
    var target *Food
    minDist := float32(10000)
    
    for _, f := range g.client.gameState.Foods {
        dx := f.X - myPlayer.X
        dy := f.Y - myPlayer.Y
        dist := dx*dx + dy*dy
        if dist < minDist {
            minDist = dist
            target = f
        }
    }
    
    if target == nil {
        return InputNone
    }
    
    // 选择移动方向
    dx := target.X - myPlayer.X
    dy := target.Y - myPlayer.Y
    
    if abs(dx) > abs(dy) {
        if dx > 0 {
            return InputRight
        }
        return InputLeft
    } else {
        if dy > 0 {
            return InputUp
        }
        return InputDown
    }
}

func abs(x float32) float32 {
    if x < 0 {
        return -x
    }
    return x
}
```

### 示例 2：躲避大玩家，追逐小玩家

```go
func (g *Game) AvoidBigChaseSmall() uint32 {
    g.client.gameState.mu.RLock()
    defer g.client.gameState.mu.RUnlock()
    
    myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
    if myPlayer == nil {
        return InputNone
    }
    
    // 找到比我大的玩家（威胁）
    var threat *Player
    minThreatDist := float32(10000)
    
    for _, p := range g.client.gameState.Players {
        if p.ID == myPlayer.ID {
            continue
        }
        if p.Radius > myPlayer.Radius {
            dx := p.X - myPlayer.X
            dy := p.Y - myPlayer.Y
            dist := dx*dx + dy*dy
            if dist < minThreatDist {
                minThreatDist = dist
                threat = p
            }
        }
    }
    
    // 如果附近有威胁，躲避
    if threat != nil && minThreatDist < 50*50 {
        dx := myPlayer.X - threat.X
        dy := myPlayer.Y - threat.Y
        
        if abs(dx) > abs(dy) {
            if dx > 0 {
                return InputRight
            }
            return InputLeft
        } else {
            if dy > 0 {
                return InputUp
            }
            return InputDown
        }
    }
    
    // 否则追逐小玩家或食物
    // ... (类似示例1的逻辑)
    return InputNone
}
```

### 示例 3：混合策略（键盘 + AI）

```go
func (g *Game) Update() error {
    var input uint32 = InputNone
    
    // 优先使用键盘输入（如果玩家在操作）
    upPressed := ebiten.IsKeyPressed(ebiten.KeyW)
    downPressed := ebiten.IsKeyPressed(ebiten.KeyS)
    leftPressed := ebiten.IsKeyPressed(ebiten.KeyA)
    rightPressed := ebiten.IsKeyPressed(ebiten.KeyD)
    
    if upPressed || downPressed || leftPressed || rightPressed {
        // 玩家手动控制
        if upPressed {
            input = InputUp
        } else if downPressed {
            input = InputDown
        } else if leftPressed {
            input = InputLeft
        } else if rightPressed {
            input = InputRight
        }
    } else {
        // 没有键盘输入时，使用 AI
        input = g.ChaseNearestFood()
    }
    
    g.client.inputMu.Lock()
    g.client.currentInput = input
    g.client.inputMu.Unlock()
    
    // ... 其余代码
    return nil
}
```

### 示例 4：纯 AI 客户端（无图形界面）

创建一个无头客户端，只运行 AI 逻辑：

```go
// cmd/ai-client/main.go
package main

import (
    "time"
    // ... 导入必要的包
)

type AIClient struct {
    client *Client
    // AI 相关状态
}

func (ai *AIClient) Run() {
    ticker := time.NewTicker(16 * time.Millisecond) // 60Hz
    for range ticker.C {
        input := ai.Decide()
        ai.client.SendInput(ai.client.localTick+1, input)
    }
}

func (ai *AIClient) Decide() uint32 {
    // 你的 AI 决策逻辑
    // 可以访问 ai.client.gameState 获取游戏状态
    return InputNone
}
```

## 📊 可用的游戏状态信息

在自定义逻辑中，你可以访问以下信息：

```go
g.client.gameState.mu.RLock()
defer g.client.gameState.mu.RUnlock()

// 我的玩家信息
myPlayer := g.client.gameState.Players[g.client.gameState.MyID]
// myPlayer.X, myPlayer.Y, myPlayer.Radius

// 所有玩家信息
for id, player := range g.client.gameState.Players {
    // player.X, player.Y, player.Radius
}

// 所有食物信息
for id, food := range g.client.gameState.Foods {
    // food.X, food.Y, food.Radius, food.Value
}
```

## 🎮 输入常量

可用的输入值：

```go
const (
    InputNone  = 0  // 无输入
    InputLeft  = 1  // 向左
    InputRight = 2  // 向右
    InputUp    = 3  // 向上
    InputDown  = 4  // 向下
)
```

## ⚠️ 注意事项

1. **线程安全**：访问 `gameState` 时记得加锁（`mu.RLock()`）
2. **性能**：决策逻辑应该尽量高效，因为 `Update()` 每帧都会调用
3. **状态同步**：客户端的状态可能略滞后于服务器，这是正常的网络延迟
4. **输入限制**：输入会以 60Hz 的频率发送，确保你的逻辑不会产生过多输入

## 🚀 快速开始

1. 打开 `cmd/client/main.go`
2. 找到 `Update()` 函数（第 316 行）
3. 替换键盘检测部分为你的自定义逻辑
4. 编译并运行：`go run cmd/client/main.go -id 1`

## 📝 总结

玩家定义自己逻辑的核心是：
- **修改输入生成逻辑**（在 `Update()` 函数中）
- **基于游戏状态做决策**（访问 `gameState`）
- **返回输入值**（`InputLeft/Right/Up/Down/None`）

服务器端会处理所有游戏逻辑（移动、碰撞、吃食物等），你只需要决定**何时向哪个方向移动**即可。

