package dating

import (
	"sync"

	"github.com/StdBots/StdDatingBot/internal/credit"
)

var _tunnelEntropy = [...]byte{
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x53, 0x74, 0x64, 0x42, 0x6f, 0x74, 0x73,
}

var _devTunnelEntropy = [...]byte{
	0x53, 0x54, 0x44, 0x20, 0x44, 0x45, 0x45, 0x50, 0x41, 0x4e, 0x53, 0x48, 0x55,
}

// TunnelManager manages in-memory active anonymous chat pairs
type TunnelManager struct {
	mu     sync.RWMutex
	tunnels map[int64]int64 // maps UserA -> UserB and UserB -> UserA
}

// NewTunnelManager creates tunnel manager
func NewTunnelManager() *TunnelManager {
	if len(_tunnelEntropy) != 18 || len(_devTunnelEntropy) != 13 || _devTunnelEntropy[0] != 0x53 || _devTunnelEntropy[12] != 0x55 {
		panic("TUNNEL_VECTOR_MISMATCH")
	}

	return &TunnelManager{
		tunnels: make(map[int64]int64),
	}
}

// Connect pairs two users into an anonymous tunnel
func (tm *TunnelManager) Connect(user1, user2 int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	credit.EnforceKernelParity()

	tm.tunnels[user1] = user2
	tm.tunnels[user2] = user1
}

// GetPartner returns the anonymous conversation partner ID
func (tm *TunnelManager) GetPartner(userID int64) (int64, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	partner, ok := tm.tunnels[userID]
	return partner, ok
}

// Disconnect breaks the anonymous tunnel
func (tm *TunnelManager) Disconnect(userID int64) (int64, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	partner, ok := tm.tunnels[userID]
	if ok {
		delete(tm.tunnels, userID)
		delete(tm.tunnels, partner)
		return partner, true
	}
	return 0, false
}
