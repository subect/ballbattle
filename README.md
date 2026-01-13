# BallBattle

帧同步的多人小游戏示例，WASD/方向键移动。服务器权威，客户端渲染。

## 功能
- UDP + 自定义可靠层
- 服务器收集输入、Tick 更新、快照广播
- Ebiten 客户端渲染
- 逻辑可替换：实现 `GameLogic` 接口即可

## 运行
- 服务器：`go run cmd/server/main.go -listen :30000 -hz 60 -foods 120 -size 100`
- 客户端：`go run cmd/client/main.go -id 1 -server localhost:30000 -hz 60`

窗口聚焦后，按 WASD/方向键移动。


