package qkchash

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
)

var errEthashStopped = errors.New("qkchash stopped")

type API struct {
	qkchash *QKCHash
}

func (api *API) GetWork() ([4]string, error) {
	var (
		workCh = make(chan [4]string, 1)
		errc   = make(chan error, 1)
	)
	select {
	case api.qkchash.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-api.qkchash.exitCh:
		return [4]string{}, errEthashStopped
	}

	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [4]string{}, err
	}
}

func (api *API) SubmitWork(nonce uint64, hash, digest common.Hash) bool {
	var errc = make(chan error, 1)

	select {
	case api.qkchash.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		errc:      errc,
	}:
	case <-api.qkchash.exitCh:
		return false
	}
	err := <-errc
	return err == nil
}
