package router

import "sync"

type Table struct {
	table map[uint32]uint64
	mutex sync.RWMutex
}

func NewTable() *Table {
	return &Table{
		table: make(map[uint32]uint64),
		mutex: sync.RWMutex{},
	}
}

func (t *Table) Update(ip uint32, steamID uint64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.table[ip] = steamID
}

// UpdateIfAbsentOrSame maps ip to steamID only if the ip is unclaimed or
// already owned by steamID. It reports whether the update was applied,
// returning false when another peer already owns the ip.
func (t *Table) UpdateIfAbsentOrSame(ip uint32, steamID uint64) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if owner, ok := t.table[ip]; ok && owner != steamID {
		return false
	}
	t.table[ip] = steamID
	return true
}

func (t *Table) Lookup(ip uint32) (uint64, bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	steamID, ok := t.table[ip]
	return steamID, ok
}

func (t *Table) Delete(ip uint32) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.table, ip)
}

func (t *Table) Snapshot() map[uint32]uint64 {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	out := make(map[uint32]uint64, len(t.table))
	for k, v := range t.table {
		out[k] = v
	}
	return out
}
