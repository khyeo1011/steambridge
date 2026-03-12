package switchboard

import (
	"sync"
)

type Table struct {
	entries map[[6]byte]uint64
	sync.RWMutex
}

func NewTable() *Table {
	return &Table{
		entries: make(map[[6]byte]uint64),
	}
}

func (t *Table) Update(mac [6]byte, steamID uint64) {
	t.Lock()
	defer t.Unlock()
	t.entries[mac] = steamID
}

func (t *Table) Lookup(mac [6]byte) (uint64, bool) {
	t.RLock()
	defer t.RUnlock()
	steamID, ok := t.entries[mac]
	return steamID, ok

}

func (t *Table) Delete(mac [6]byte) {
	t.Lock()
	defer t.Unlock()
	delete(t.entries, mac)
}

func (t *Table) Forget(steamID uint64) {
	t.Lock()
	defer t.Unlock()
	for mac, id := range t.entries {
		if id == steamID {
			delete(t.entries, mac)
		}
	}
}
