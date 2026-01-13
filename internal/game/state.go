package game

import (
	"math/rand"
	"sync"
	"time"
)

// Input constants (aligned with gameframework demo)
const (
	InputNone  = 0
	InputLeft  = 1
	InputRight = 2
	InputUp    = 3
	InputDown  = 4
)

type Player struct {
	ID     uint16
	X      float32
	Y      float32
	Radius float32
}

type Food struct {
	ID     uint32
	X      float32
	Y      float32
	Value  float32
	Radius float32
}

// State holds world state.
type State struct {
	mu        sync.Mutex
	Players   map[uint16]*Player
	Foods     map[uint32]*Food
	arenaHalf float32
	rng       *rand.Rand
}

func NewState(arenaHalf float32, foodCount int) *State {
	s := &State{
		Players:   make(map[uint16]*Player),
		Foods:     make(map[uint32]*Food),
		arenaHalf: arenaHalf,
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	for i := 0; i < foodCount; i++ {
		s.spawnFood()
	}
	return s
}

// Spawn a player at random position.
func (s *State) AddPlayer(id uint16) *Player {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := &Player{
		ID:     id,
		X:      s.randInRange(),
		Y:      s.randInRange(),
		Radius: 1.2,
	}
	s.Players[id] = p
	return p
}

func (s *State) RemovePlayer(id uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Players, id)
}

func (s *State) randInRange() float32 {
	return (s.rng.Float32()*2 - 1) * s.arenaHalf
}

// ApplyInput moves player and checks food eats.
func (s *State) ApplyInput(pid uint16, input uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.Players[pid]
	if !ok {
		// late join safety
		p = &Player{ID: pid, X: s.randInRange(), Y: s.randInRange(), Radius: 1.2}
		s.Players[pid] = p
	}

	speedFactor := 1.5 / (1.0 + float32(p.Radius))
	if speedFactor < 0.4 {
		speedFactor = 0.4
	}
	speed := float32(2.0) * speedFactor // bigger slower (increased from 0.8 to 2.0 for faster movement)
	switch input {
	case InputLeft:
		p.X -= speed
	case InputRight:
		p.X += speed
	case InputUp:
		p.Y += speed
	case InputDown:
		p.Y -= speed
	}
	// clamp to arena
	p.X = clamp(p.X, -s.arenaHalf, s.arenaHalf)
	p.Y = clamp(p.Y, -s.arenaHalf, s.arenaHalf)

	// eat foods
	for id, f := range s.Foods {
		if collide(p.X, p.Y, p.Radius, f.X, f.Y, f.Radius) {
			p.Radius += f.Value
			delete(s.Foods, id)
			s.spawnFood()
		}
	}
}

func (s *State) spawnFood() {
	id := uint32(len(s.Foods) + 1 + int(s.rng.Int31()))
	s.Foods[id] = &Food{
		ID:     id,
		X:      s.randInRange(),
		Y:      s.randInRange(),
		Value:  0.15,
		Radius: 0.35,
	}
}

// Snapshot returns copies for broadcast.
type Snapshot struct {
	Players []*Player
	Foods   []*Food
}

func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Snapshot{
		Players: make([]*Player, 0, len(s.Players)),
		Foods:   make([]*Food, 0, len(s.Foods)),
	}
	for _, p := range s.Players {
		cp := *p
		out.Players = append(out.Players, &cp)
	}
	for _, f := range s.Foods {
		cf := *f
		out.Foods = append(out.Foods, &cf)
	}
	return out
}

func clamp(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func collide(x1, y1, r1, x2, y2, r2 float32) bool {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	rr := float64(r1 + r2)
	return dx*dx+dy*dy <= rr*rr
}
