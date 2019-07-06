package test

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

/*func TestShardGenesisForFork(t *testing.T) {
	clstrList := CreateClusterList(2, 1, 1, 1)
	clstrList.Start()
	defer clstrList.Stop()
	mstr := clstrList[0].GetMaster()
	root1, err  := mstr.CreateBlockToMine()
	if err != nil {
		utils.Fatalf("failed to create root block: %v", err)
	}
}*/

func TestGetRootBlockHeaderSyncWithFork(t *testing.T) {
	clstrList := CreateClusterList(2, 1, 1, 1)
	clstrList.Start()
	defer clstrList.Stop()
	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
	)
	for i := 0; i < 10; i++ {
		iBlock, err := mstr0.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr0.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
		rootBlockList = append(rootBlockList, rBlock)
	}
	for i := 0; i < 2; i++ {
		if err := mstr1.AddRootBlock(rootBlockList[i]); err != nil {
			assert.Error(t, err)
		}
	}
	for i := 0; i < 3; i++ {
		iBlock, err := mstr1.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr1.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
	}
}
