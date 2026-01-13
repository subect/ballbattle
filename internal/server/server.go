package server

import (
	"time"

	"ballbattle/internal/game"
	"gameframework/pkg/netcore"
)

// Server 封装 netcore.Server，简化接口
type Server struct {
	netcore *netcore.Server
}

// New 创建服务器，使用 netcore 封装。
// 默认使用 NewServerWithConfig，以适配最新的网络框架。
func New(listen string, tickHz int, foodCount int, arenaHalf float32) (*Server, error) {
	// 创建游戏状态
	state := game.NewState(arenaHalf, foodCount)

	// 创建游戏逻辑
	logic := game.NewBallBattleLogic(state)

	// 使用新版本的 ServerConfig 接口创建服务器
	cfg := netcore.ServerConfig{
		ListenAddr:    listen,
		TickHz:        tickHz,
		PlayerTimeout: 30 * time.Second,
		// 每秒最多处理的输入数，留空则走 netcore 默认值；
		// 这里显式设置，便于以后调优。
		MaxInputPerSec: 100,
	}

	netcoreSrv, err := netcore.NewServerWithConfig(cfg, logic)
	if err != nil {
		return nil, err
	}

	return &Server{netcore: netcoreSrv}, nil
}

// Start 启动所有服务器循环（推荐使用）
func (s *Server) Start() {
	s.netcore.Start()
}

// Close 关闭底层 UDP 连接
func (s *Server) Close() error {
	return s.netcore.Close()
}

// ListenLoop 接收循环
func (s *Server) ListenLoop() {
	s.netcore.ListenLoop()
}

// BroadcastLoop 广播循环
func (s *Server) BroadcastLoop() {
	s.netcore.BroadcastLoop()
}

// ReliableRetransmitLoop 可靠重传循环
func (s *Server) ReliableRetransmitLoop() {
	s.netcore.ReliableRetransmitLoop()
}

// CheckPlayerTimeout 检测玩家超时并清理
func (s *Server) CheckPlayerTimeout() {
	s.netcore.CheckPlayerTimeout()
}
