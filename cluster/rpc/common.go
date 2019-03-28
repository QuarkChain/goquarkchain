package rpc

type ServerType int

const (
	_ = iota
	OpPing
	OpConnectToSlaves
	OpAddRootBlock
	OpGetEcoInfoList
	OpGetNextBlockToMine
	OpGetUnconfirmedHeaders
	OpGetAccountData
	OpAddTransaction
	OpAddMinorBlockHeader
	OpAddXshardTxList
	OpSyncMinorBlockList
	OpAddMinorBlock
	OpCreateClusterPeerConnection
	OpDestroyClusterPeerConnectionCommand
	OpGetMinorBlock
	OpGetTransaction
	OpBatchAddXshardTxList
	OpExecuteTransaction
	OpGetTransactionReceipt
	OpGetMine
	OpGenTx
	OpGetTransactionListByAddress
	OpGetLogs
	OpEstimateGas
	OpGetStorageAt
	OpGetCode
	OpGasPrice
	OpGetWork
	OpSubmitWork

	MasterServer  = ServerType(1)
	SlaveServer   = ServerType(0)
	UnknownServer = ServerType(-1)
)

var (
	// include all grpc funcs
	rpcFuncs = map[int64]opType{
		OpPing:                                {name: "Ping"},
		OpConnectToSlaves:                     {name: "ConnectToSlaves"},
		OpAddRootBlock:                        {name: "AddRootBlock"},
		OpGetEcoInfoList:                      {name: "GetEcoInfoList"},
		OpGetNextBlockToMine:                  {name: "GetNextBlockToMine"},
		OpGetUnconfirmedHeaders:               {name: "GetUnconfirmedHeaders"},
		OpGetAccountData:                      {name: "GetAccountData"},
		OpAddTransaction:                      {name: "AddTransaction"},
		OpAddMinorBlockHeader:                 {name: "AddMinorBlockHeader", serverType: MasterServer},
		OpAddXshardTxList:                     {name: "AddXshardTxList"},
		OpSyncMinorBlockList:                  {name: "SyncMinorBlockList"},
		OpAddMinorBlock:                       {name: "AddMinorBlock"},
		OpCreateClusterPeerConnection:         {name: "CreateClusterPeerConnection"},
		OpDestroyClusterPeerConnectionCommand: {name: "DestroyClusterPeerConnectionCommand", serverType: UnknownServer},
		OpGetMinorBlock:                       {name: "GetMinorBlock"},
		OpGetTransaction:                      {name: "GetTransaction"},
		OpBatchAddXshardTxList:                {name: "BatchAddXshardTxList"},
		OpExecuteTransaction:                  {name: "ExecuteTransaction"},
		OpGetTransactionReceipt:               {name: "GetTransactionReceipt"},
		OpGetMine:                             {name: "GetMine"},
		OpGenTx:                               {name: "GenTx"},
		OpGetTransactionListByAddress:         {name: "GetTransactionListByAddress"},
		OpGetLogs:                             {name: "GetLogs"},
		OpEstimateGas:                         {name: "EstimateGas"},
		OpGetStorageAt:                        {name: "GetStorageAt"},
		OpGetCode:                             {name: "GetCode"},
		OpGasPrice:                            {name: "GasPrice"},
		OpGetWork:                             {name: "GetWork"},
		OpSubmitWork:                          {name: "SubmitWork"},
	}
)
