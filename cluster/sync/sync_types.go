package sync

type RootBlockSychronizerStats struct {
	HeadersDownloaded      uint64 `json:"headers_downloaded" gencodec:"required"`
	BlocksDownloaded       uint64 `json:"blocks_downloaded" gencodec:"required"`
	BlocksAdded            uint64 `json:"blocks_added" gencodec:"required"`
	AncestorNotFoundCount  uint64 `json:"ancestor_not_found_count" gencodec:"required"`
	AncestorLookupRequests uint64 `json:"ancestor_lookup_requests" gencodec:"required"`
}

type MinorBlockSychronizerStats struct {
	HeadersDownloaded      uint64 `json:"headers_downloaded" gencodec:"required"`
	BlocksDownloaded       uint64 `json:"blocks_downloaded" gencodec:"required"`
	BlocksAdded            uint64 `json:"blocks_added" gencodec:"required"`
	AncestorNotFoundCount  uint64 `json:"ancestor_not_found_count" gencodec:"required"`
	AncestorLookupRequests uint64 `json:"ancestor_lookup_requests" gencodec:"required"`
}
