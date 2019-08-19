package qkcapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
)

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From            *account.Address `json:"from"`
	To              *account.Address `json:"to"`
	Gas             hexutil.Big      `json:"gas"`
	GasPrice        hexutil.Big      `json:"gasPrice"`
	Value           hexutil.Big      `json:"value"`
	Data            hexutil.Bytes    `json:"data"`
	GasTokenID      *hexutil.Uint64  `json:"gas_token_id"`
	TransferTokenID *hexutil.Uint64  `json:"transfer_token_id"`
}

func (c *CallArgs) setDefaults() {
	if c.From == nil {
		temp := account.CreatEmptyAddress(c.To.FullShardKey)
		c.From = &temp
	}
}
func (c *CallArgs) toTx(config *config.QuarkChainConfig) (*types.Transaction, error) {
	gasTokenID, transferTokenID := config.GetDefaultChainTokenID(), config.GetDefaultChainTokenID()
	if c.GasTokenID == nil {
		gasTokenID = uint64(*c.GasTokenID)
	}
	if c.TransferTokenID == nil {
		transferTokenID = uint64(*c.TransferTokenID)
	}
	evmTx := types.NewEvmTransaction(0, c.To.Recipient, c.Value.ToInt(), c.Gas.ToInt().Uint64(),
		c.GasPrice.ToInt(), c.From.FullShardKey, c.To.FullShardKey, config.NetworkID, 0, c.Data, gasTokenID, transferTokenID)
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx, nil
}

type CreateTxArgs struct {
	NumTxPreShard    *hexutil.Uint   `json:"numTxPerShard"`
	XShardPrecent    *hexutil.Uint   `json:"xShardPercent"`
	To               *common.Address `json:"to"`
	Gas              *hexutil.Big    `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Value            *hexutil.Big    `json:"value"`
	Data             *hexutil.Bytes  `json:"data"`
	FromFullShardKey *hexutil.Uint   `json:"fromFullShardKey"`
	GasTokenID       *hexutil.Uint64 `json:"gas_token_id"`
	TransferTokenID  *hexutil.Uint64 `json:"transfer_token_id"`
}

func (c *CreateTxArgs) setDefaults(config *config.QuarkChainConfig) error {
	if c.NumTxPreShard == nil {
		return errors.New("must set numTxPerShard")
	}
	if c.XShardPrecent == nil {
		return errors.New("must set xShardPercent")
	}
	if c.Gas == nil {
		c.Gas = (*hexutil.Big)(params.DefaultStartGas)
	}
	if c.GasPrice == nil {
		c.GasPrice = (*hexutil.Big)(new(big.Int).Div(params.DenomsValue.GWei, new(big.Int).SetUint64(10)))
	}
	if c.Value == nil {
		t := hexutil.Big{}
		c.Value = &t
	}
	if c.Data == nil {
		t := hexutil.Bytes{}
		c.Data = &t
	}
	if c.FromFullShardKey == nil {
		t := hexutil.Uint(0)
		c.FromFullShardKey = &t
	}
	if c.GasTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		c.GasTokenID = &t
	}
	if c.TransferTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		c.TransferTokenID = &t
	}
	return nil
}
func (c *CreateTxArgs) toTx(config *config.QuarkChainConfig) *types.Transaction {
	var (
		evmTx *types.EvmTransaction
	)
	if c.To == nil {
		evmTx = types.NewEvmContractCreation(0, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(),
			uint32(*c.FromFullShardKey), 0, config.NetworkID, 0, *c.Data, uint64(*c.GasTokenID), uint64(*c.TransferTokenID))
	} else {
		evmTx = types.NewEvmTransaction(0, *c.To, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(),
			uint32(*c.FromFullShardKey), 0, config.NetworkID, 0, *c.Data, uint64(*c.GasTokenID), uint64(*c.TransferTokenID))
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	To       *common.Address `json:"to"`
	Gas      *hexutil.Big    `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data             *hexutil.Bytes  `json:"data"`
	FromFullShardKey *hexutil.Uint   `json:"fromFullShardKey"`
	ToFullShardKey   *hexutil.Uint   `json:"toFullShardKey"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
	NetWorkID        *hexutil.Uint   `json:"networkId"`
	GasTokenID       *hexutil.Uint64 `json:"gas_token_id"`
	TransferTokenID  *hexutil.Uint64 `json:"transfer_token_id"`
}

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults(config *config.QuarkChainConfig) error {
	if args.Gas == nil {
		args.Gas = (*hexutil.Big)(params.DefaultStartGas)
	}
	if args.GasPrice == nil {
		args.GasPrice = (*hexutil.Big)(params.DefaultGasPrice)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Data == nil {
		args.Data = new(hexutil.Bytes)
	}
	if args.Nonce == nil {
		return errors.New("nonce is missing")
	}
	if args.FromFullShardKey == nil {
		return errors.New("fromFullShardKey is missing")
	}
	if args.ToFullShardKey == nil {
		args.ToFullShardKey = args.FromFullShardKey
	}

	if args.NetWorkID != nil {
		t := hexutil.Uint(config.NetworkID)
		args.NetWorkID = &t
	}

	if args.GasTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		args.GasTokenID = &t
	}
	if args.TransferTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		args.TransferTokenID = &t
	}

	if args.V == nil || args.R == nil || args.S == nil {
		return errors.New("missing v r s")
	}
	return nil
}

func (args *SendTxArgs) toTransaction() (*types.Transaction, error) {
	var evmTx *types.EvmTransaction
	if args.To == nil {
		evmTx = types.NewEvmContractCreation(uint64(*args.Nonce), (*big.Int)(args.Value), args.Gas.ToInt().Uint64(),
			(*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey), uint32(*args.ToFullShardKey),
			uint32(*args.NetWorkID), 0, *args.Data, uint64(*args.GasTokenID), uint64(*args.TransferTokenID))
	} else {
		evmTx = types.NewEvmTransaction(uint64(*args.Nonce), account.BytesToIdentityRecipient(args.To.Bytes()),
			(*big.Int)(args.Value), args.Gas.ToInt().Uint64(), (*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey),
			uint32(*args.ToFullShardKey), uint32(*args.NetWorkID), 0, *args.Data, uint64(*args.GasTokenID), uint64(*args.TransferTokenID))
	}

	evmTx.SetVRS(args.V.ToInt(), args.R.ToInt(), args.S.ToInt())

	return &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}, nil
}

type FilterQuery struct {
	FromBlock *big.Int          // beginning of the queried range, nil means genesis block
	ToBlock   *big.Int          // end of the range, nil means latest block
	Addresses []account.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}, {B}}         matches topic A in first position, B in second position
	// {{A, B}, {C, D}}   matches topic (A OR B) in first position, (C OR D) in second position
	Topics [][]common.Hash
}

//https://github.com/ethereum/go-ethereum/blob/v1.8.20/eth/filters/api.go line 460 //TODO delete it
// UnmarshalJSON sets *args fields with given data.
func (args *FilterQuery) UnmarshalJSON(data []byte) error {
	type input struct {
		FromBlock *rpc.BlockNumber `json:"fromBlock"`
		ToBlock   *rpc.BlockNumber `json:"toBlock"`
		Addresses interface{}      `json:"address"`
		Topics    []interface{}    `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.FromBlock != nil {
		args.FromBlock = big.NewInt(raw.FromBlock.Int64())
	}

	if raw.ToBlock != nil {
		args.ToBlock = big.NewInt(raw.ToBlock.Int64())
	}

	args.Addresses = make([]account.Address, 0)

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr, err := decodeAddress(strAddr)
					if err != nil {
						return fmt.Errorf("invalid address at index %d: %v", i, err)
					}
					args.Addresses = append(args.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr, err := decodeAddress(rawAddr)
			if err != nil {
				return fmt.Errorf("invalid address: %v", err)
			}
			args.Addresses = []account.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		args.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top, err := decodeTopic(topic)
				if err != nil {
					return err
				}
				args.Topics[i] = []common.Hash{top}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed, err := decodeTopic(topic)
						if err != nil {
							return err
						}
						args.Topics[i] = append(args.Topics[i], parsed)
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}
func decodeAddress(s string) (account.Address, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.AddressLength+4 {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for address", len(b), common.AddressLength)
	}
	return account.CreatAddressFromBytes(b)
}

func decodeTopic(s string) (common.Hash, error) {
	b, err := hexutil.Decode(s)
	if err == nil && len(b) != common.HashLength {
		err = fmt.Errorf("hex has invalid length %d after decoding; expected %d for topic", len(b), common.HashLength)
	}
	return common.BytesToHash(b), err
}
