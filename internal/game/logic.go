package game

import (
	"encoding/binary"
	"bytes"
	"net"
)

// BallBattleLogic 实现 gameframework 的 GameLogic 接口
type BallBattleLogic struct {
	state *State
}

func NewBallBattleLogic(state *State) *BallBattleLogic {
	return &BallBattleLogic{state: state}
}

// OnJoin 玩家加入时初始化
func (l *BallBattleLogic) OnJoin(pid uint16) {
	l.state.AddPlayer(pid)
}

// OnLeave 玩家离开（预留）
func (l *BallBattleLogic) OnLeave(pid uint16) {
	l.state.RemovePlayer(pid)
}

// ApplyInput 收到输入时立即应用（用于即时反馈，可选）
func (l *BallBattleLogic) ApplyInput(pid uint16, input uint32) {
	// 这里可以提前处理输入，但主要逻辑在 Tick 中
}

// Tick 每个 tick 调用，应用所有输入并更新游戏状态
func (l *BallBattleLogic) Tick(tick uint32, inputs map[uint16]uint32) {
	for pid, input := range inputs {
		if input != 0 { // 只处理非零输入（InputNone=0 不需要处理）
			l.state.ApplyInput(pid, input)
		}
	}
}

// Snapshot 返回当前状态的二进制快照
// 格式: uint8 playerCount, [pid(uint16), x(float32), y(float32), radius(float32)]*N,
//       uint16 foodCount, [id(uint32), x(float32), y(float32), value(float32), radius(float32)]*M
func (l *BallBattleLogic) Snapshot(tick uint32) ([]byte, error) {
	snap := l.state.Snapshot()
	buf := &bytes.Buffer{}
	
	// players section
	binary.Write(buf, binary.LittleEndian, uint8(len(snap.Players)))
	for _, p := range snap.Players {
		binary.Write(buf, binary.LittleEndian, p.ID)
		binary.Write(buf, binary.LittleEndian, p.X)
		binary.Write(buf, binary.LittleEndian, p.Y)
		binary.Write(buf, binary.LittleEndian, p.Radius)
	}
	
	// foods section
	binary.Write(buf, binary.LittleEndian, uint16(len(snap.Foods)))
	for _, f := range snap.Foods {
		binary.Write(buf, binary.LittleEndian, f.ID)
		binary.Write(buf, binary.LittleEndian, f.X)
		binary.Write(buf, binary.LittleEndian, f.Y)
		binary.Write(buf, binary.LittleEndian, f.Value)
		binary.Write(buf, binary.LittleEndian, f.Radius)
	}
	
	return buf.Bytes(), nil
}

// HandleReliableMessage 处理可靠消息
// 对于 ballbattle 游戏，不需要特殊的可靠消息处理
// 返回 false 表示不处理该消息，框架会使用默认处理
func (l *BallBattleLogic) HandleReliableMessage(peerID uint16, addr *net.UDPAddr, msgType byte, payload []byte) (handled bool, playerID int) {
	// ballbattle 游戏不需要特殊的可靠消息处理
	// 玩家通过发送输入包自动注册
	return false, 0
}
