package server

import (
	"ballbattle/internal/game"
	"gameframework/pkg/netcore"
)

// Server 封装 netcore.Server，简化接口
type Server struct {
	netcore *netcore.Server
}

// New 创建服务器，使用 netcore 封装
func New(listen string, tickHz int, foodCount int, arenaHalf float32) (*Server, error) {
	// 创建游戏状态
	state := game.NewState(arenaHalf, foodCount)
	
	// 创建游戏逻辑
	logic := game.NewBallBattleLogic(state)
	
	// 使用 netcore.Server 处理所有网络层
	netcoreSrv, err := netcore.NewServer(listen, tickHz, logic)
	if err != nil {
		return nil, err
	}
	
	return &Server{netcore: netcoreSrv}, nil
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
