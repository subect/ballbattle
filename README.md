# BallBattle

帧同步的多人小游戏示例，服务器权威。

## 功能
- UDP + 自定义可靠层
- 服务器收集输入、Tick 更新、快照广播
- Ebiten 客户端渲染
- 逻辑可替换：实现 `GameLogic` 接口即可

## 运行
- 服务器：`go run cmd/server/main.go -listen :30000 -hz 60 -foods 120 -size 100`

本仓库仅包含服务器实现；客户端可按需自行实现（例如 Unity）。


