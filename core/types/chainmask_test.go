package types

import "testing"

func TestShardMask(t *testing.T) {
	var (
		sm0Data = make(map[uint32]bool)
		sm1Data = make(map[uint32]bool)
	)

	sm0 := NewChainMask(0x01)
	sm0Data[0x00] = true
	sm0Data[0x1<<16] = true
	sm0Data[0x7f] = true
	for fullShardId, ok := range sm0Data {
		if !sm0.ContainFullShardId(fullShardId) == ok {
			t.Errorf("full shard id is not exist: %d %d", 0x01, fullShardId)
		}
	}

	sm1 := NewChainMask(0x05)
	sm1Data[0x00] = false
	sm1Data[0x1<<16] = true
	sm1Data[0x2<<16] = false
	sm1Data[0x3<<16] = false
	sm1Data[0x4<<16] = false
	sm1Data[0x5<<16] = true
	sm1Data[0x6<<16] = false
	sm1Data[0x7<<16] = false
	sm1Data[0x8<<16] = false
	sm1Data[0x9<<16] = true
	for fullShardId, ok := range sm1Data {
		if !sm1.ContainFullShardId(fullShardId) == ok {
			t.Errorf("full shard id is not exist: %d %d", 0x05, fullShardId)
		}
	}
}
