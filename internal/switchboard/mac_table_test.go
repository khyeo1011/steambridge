package switchboard

import (
	"sync"
	"testing"
)

func TestTable_UpdateAndLookup(t *testing.T) {
	table := NewTable()
	macA := [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	steamIDA := uint64(123456789)

	if _, ok := table.Lookup(macA); ok {
		t.Error("Lookup succeeded for unknown MAC, expected failure")
	}

	table.Update(macA, steamIDA)
	if got, ok := table.Lookup(macA); !ok || got != steamIDA {
		t.Errorf("Lookup failed. Expected %d, got %d", steamIDA, got)
	}
}

func TestTable_Forget(t *testing.T) {
	table := NewTable()
	mac1 := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	mac2 := [6]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x66}
	steamID := uint64(987654321)

	table.Update(mac1, steamID)
	table.Update(mac2, steamID)

	table.Forget(steamID)

	if _, ok := table.Lookup(mac1); ok {
		t.Error("mac1 was not forgotten")
	}
	if _, ok := table.Lookup(mac2); ok {
		t.Error("mac2 was not forgotten")
	}
}

func TestTable_Concurrency(t *testing.T) {
	table := NewTable()
	mac := [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(id uint64) {
			defer wg.Done()
			table.Update(mac, id)
		}(uint64(i))

		go func() {
			defer wg.Done()
			table.Lookup(mac)
		}()
	}

	wg.Wait()
}
