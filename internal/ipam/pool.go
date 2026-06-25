package ipam

import (
	"sync"
)

type Pool struct {
	mu          sync.Mutex
	baseIP      uint32
	hostCounter uint32
	leases      map[uint64]uint32
	// TODO(human): add a free-list field here to track released host numbers
	// so Allocate can reuse them before incrementing hostCounter.
	// Consider: []uint32 slice-as-stack (LIFO) vs a queue (FIFO).
	released []uint32
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

	if len(p.released) > 0 {
		ip := p.released[0]
		p.released = p.released[1:]
		p.leases[steamID] = ip
		return ip
	}

	ip := p.baseIP | p.hostCounter
	p.leases[steamID] = ip
	p.hostCounter++
	return ip
}

func (p *Pool) Release(ip uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for steamID, lease := range p.leases {
		if lease == ip {
			delete(p.leases, steamID)
			p.released = append(p.released, ip)
			return
		}
	}
}
