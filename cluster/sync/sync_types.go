package sync

type BlockSychronizerStats struct {
	HeadersDownloaded      uint64 `json:"headers_downloaded" gencodec:"required"`
	BlocksDownloaded       uint64 `json:"blocks_downloaded" gencodec:"required"`
	BlocksAdded            uint64 `json:"blocks_added" gencodec:"required"`
	AncestorNotFoundCount  uint64 `json:"ancestor_not_found_count" gencodec:"required"`
	AncestorLookupRequests uint64 `json:"ancestor_lookup_requests" gencodec:"required"`
}

type SyncProgress struct {
	// StartingBlock uint64 // Block number where sync began
	CurrentBlock uint64 // Current block number where sync is at
	HighestBlock uint64 // Highest alleged block number in the chain
	// PulledStates  uint64 // Number of state trie entries already downloaded
	// KnownStates   uint64 // Total number of state trie entries known about
}

// SyncingResult provides information about the current synchronisation status for this node.
type SyncingResult struct {
	Syncing bool         `json:"syncing"`
	Status  SyncProgress `json:"status"`
}
