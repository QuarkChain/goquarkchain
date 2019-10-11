package core

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"sync/atomic"
)

type taskData struct {
	errc chan error
	tx   *types.Transaction
}
type senderCacheService struct {
	tasks        chan *taskData
	numThread    int
	closeChans   chan struct{}
	stopFlag     int32
	loopStopChan chan struct{}

	networkID uint32
	addTxFunc func(tx *types.Transaction) error
}

func NewService(capacity int, networkID uint32, addTxFunc func(tx *types.Transaction) error) *senderCacheService {
	service := &senderCacheService{}
	service.numThread = 4
	service.tasks = make(chan *taskData, capacity)
	service.stopFlag = 0
	service.closeChans = make(chan struct{}, service.numThread)
	service.loopStopChan = make(chan struct{})
	service.addTxFunc = addTxFunc
	service.networkID = networkID
	service.Run()
	return service
}

func (this *senderCacheService) Stop() {
	atomic.StoreInt32(&this.stopFlag, 1)
	<-this.loopStopChan
	close(this.tasks)
	for i := 0; i < this.numThread; i++ {
		<-this.closeChans
	}
}

func (this *senderCacheService) Run() {
	for i := 0; i < this.numThread; i++ {
		go this.run(i)
	}
}

func (this *senderCacheService) run(i int) {
loop:
	for {
		select {
		case data, ok := <-this.tasks:
			if ok {
				_, err := types.Sender(types.NewEIP155Signer(this.networkID), data.tx.EvmTx)
				if err != nil {
					panic(err)
					//break loop
				}
				//fmt.Println("ready tp add tx", data.tx.EvmTx.IsFromNil(), data.tx.EvmTx.Hash().String())
				//if err := this.addTxFunc(data.tx); err != nil {
				//	panic(err)
				//}
				//fmt.Println("ready tp add tx-end", data.tx.EvmTx.IsFromNil(), data.tx.EvmTx.Hash().String())

				if data.tx.EvmTx.IsFromNil() {
					panic("scf sb")
				}
				data.errc <- err
			} else {
				break loop
			}
		}
	}
	this.closeChans <- struct{}{}
}

func (this *senderCacheService) AddTx(tx *types.Transaction) error {
	errc := make(chan error, 1)
	this.tasks <- &taskData{
		tx:   tx,
		errc: errc,
	}
	select {
	case err := <-errc:
		return err
	}
}
