package ipam

import (
	"sync"
)

// maxHost is the highest usable host number in the 10.8.0.0/24 subnet.
// .0 is the network address, .1 is the host, .255 is broadcast, so guests
// get 10.8.0.2 through 10.8.0.254.
const maxHost = 254

type Pool struct {
	mu          sync.Mutex
	baseIP      uint32
	hostCounter uint32
	leases      map[uint64]uint32
	// released holds freed IPs, reused FIFO before advancing hostCounter.
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

// Allocate returns the IP leased to steamID, granting a new lease if needed.
// Returns 0 when the pool is exhausted (all of 10.8.0.2-10.8.0.254 leased
// and nothing on the free-list).
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

	if p.hostCounter > maxHost {
		return 0
	}

	ip := p.baseIP | p.hostCounter
	p.leases[steamID] = ip
	p.hostCounter++
	return ip
}

// ReleaseBySteamID frees the lease owned by steamID, returning the freed IP
// and true, or (0, false) if steamID holds no lease. Peers can only ever free
// their own lease this way, unlike releasing by an attacker-supplied IP.
func (p *Pool) ReleaseBySteamID(steamID uint64) (uint32, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ip, ok := p.leases[steamID]
	if !ok {
		return 0, false
	}
	delete(p.leases, steamID)
	p.released = append(p.released, ip)
	return ip, true
}
