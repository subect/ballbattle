# BallBattle

帧同步的多人小游戏示例，服务器权威。

## 功能
- UDP + 自定义可靠层
- 服务器收集输入、Tick 更新、快照广播
- Ebiten 客户端渲染
- 逻辑可替换：实现 `GameLogic` 接口即可

## 运行
- 服务器：`go run cmd/server/main.go -listen :30000 -hz 60 -foods 120 -size 100`
- 客户端：`go run cmd/client/main.go -server localhost:30000 -id 1 -hz 60 -size 100`

### 客户端功能
客户端使用 `gameframework/pkg/client` 包，提供了完整的 Ebiten 渲染界面：
- **网络层**：自动处理 UDP 连接、可靠传输、ping/pong
- **渲染**：实时显示所有玩家和食物，支持多窗口同时运行
- **输入**：WASD 或方向键控制移动
- **统计**：显示服务器 tick、RTT、丢包率、玩家数量等信息

### 多玩家测试
可以同时运行多个客户端窗口进行测试：
```bash
# 终端1：服务器
go run cmd/server/main.go -listen :30000 -hz 60 -foods 120 -size 100

# 终端2：玩家1
go run cmd/client/main.go -server localhost:30000 -id 1 -hz 60 -size 100

# 终端3：玩家2
go run cmd/client/main.go -server localhost:30000 -id 2 -hz 60 -size 100
```

窗口聚焦后，按 WASD/方向键移动。玩家会显示为不同颜色的圆圈，食物显示为绿色小点。


