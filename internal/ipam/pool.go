package ipam

import (
	"fmt"
	"sync"
)

type Pool struct {
	mu          sync.Mutex
	baseIP      uint32
	hostCounter uint32
	leases      map[uint64]uint32
}

func NewPool() *Pool {
	var base uint32 = (10 << 24) | (8 << 16) | (0 << 8)

	return &Pool{
		baseIP:      base,
		hostCounter: 2,
		leases:      make(map[uint64]uint32),
	}
}

func (p *Pool) Allocate(steamID uint64) uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()

	if existingIP, ok := p.leases[steamID]; ok {
		return existingIP
	}
	ip := p.baseIP | p.hostCounter
	p.leases[steamID] = ip
	p.hostCounter++
	return ip
}

func IntIPtoString(ip uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", ip>>24, (ip>>16)&0xFF, (ip>>8)&0xFF, ip&0xFF)
}

func (p *Pool) Release(ip uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for steamID, lease := range p.leases {
		if lease == ip {
			delete(p.leases, steamID)
			return
		}
	}
}

func (p *Pool) RealeaseSteamID(steamID uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.leases, steamID)
}
