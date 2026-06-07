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
