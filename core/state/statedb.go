// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package state provides a caching layer atop the Ethereum state trie.
package state

import (
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/params"

	qkcaccount "github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

type proofList [][]byte

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}

// StateDBs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	thash, bhash common.Hash
	txIndex      int
	logs         map[common.Hash][]*types.Log
	logSize      uint

	preimages map[common.Hash][]byte

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int

	xShardReceiveGasUsed *big.Int
	blockFee             map[uint64]*big.Int
	xShardList           []*types.CrossShardTransactionDeposit
	fullShardKey         uint32
	quarkChainConfig     *config.QuarkChainConfig
	gasUsed              *big.Int
	gasLimit             *big.Int
	shardConfig          *config.ShardConfig
	senderDisallowMap    map[qkcaccount.Recipient]*big.Int
	blockCoinbase        common.Address
	timeStamp            uint64
	blockNumber          uint64
	xShardTxCursorInfo   *types.XShardTxCursorInfo
	useMock              bool
}

// Create a new state from a given trie.
func New(root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	stateDB := &StateDB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
	}
	stateDB.SetGasLimit(params.DefaultStateDBGasLimit)
	return stateDB, nil

}

// setError remembers the first non-nil error it is called with.
func (s *StateDB) setError(err error) {
	if s.dbErr == nil {
		s.dbErr = err
	}
}

func (s *StateDB) Error() error {
	return s.dbErr
}

// reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (s *StateDB) Reset(root common.Hash) error {
	tr, err := s.db.OpenTrie(root)
	if err != nil {
		return err
	}
	s.trie = tr
	s.stateObjects = make(map[common.Address]*stateObject)
	s.stateObjectsDirty = make(map[common.Address]struct{})
	s.thash = common.Hash{}
	s.bhash = common.Hash{}
	s.txIndex = 0
	s.logs = make(map[common.Hash][]*types.Log)
	s.logSize = 0
	s.preimages = make(map[common.Hash][]byte)
	s.clearJournalAndRefund()
	return nil
}

func (s *StateDB) AddLog(log *types.Log) {
	s.journal.append(addLogChange{txhash: s.thash})

	log.TxHash = s.thash
	log.BlockHash = s.bhash
	log.TxIndex = uint32(s.txIndex)
	log.Index = uint32(s.logSize)
	s.logs[s.thash] = append(s.logs[s.thash], log)
	s.logSize++
}

func (s *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return s.logs[hash]
}

func (s *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range s.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// AddPreimage records a SHA3 preimage seen by the VM.
func (s *StateDB) AddPreimage(hash common.Hash, preimage []byte) {
	if _, ok := s.preimages[hash]; !ok {
		s.journal.append(addPreimageChange{hash: hash})
		pi := make([]byte, len(preimage))
		copy(pi, preimage)
		s.preimages[hash] = pi
	}
}

// Preimages returns a list of SHA3 preimages that have been submitted.
func (s *StateDB) Preimages() map[common.Hash][]byte {
	return s.preimages
}

// AddRefund adds gas to the refund counter
func (s *StateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *StateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic("Refund counter below zero")
	}
	s.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (s *StateDB) Exist(addr common.Address) bool {
	return s.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (s *StateDB) Empty(addr common.Address) bool {
	so := s.getStateObject(addr)
	return so == nil || so.empty()
}

// Retrieve the balance from the given address or 0 if object not found
func (s *StateDB) GetBalance(addr common.Address, tokenID uint64) *big.Int {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance(tokenID)
	}
	return common.Big0
}

func (s *StateDB) GetBalances(addr common.Address) *types.TokenBalances {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		return stateObject.data.TokenBalances.Copy()
	}
	return types.NewEmptyTokenBalances()
}

func (s *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

func (s *StateDB) GetCode(addr common.Address) []byte {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(s.db)
	}
	return nil
}

func (s *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := s.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		s.setError(err)
	}
	return size
}

func (s *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
func (s *StateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(s.db, hash)
	}
	return common.Hash{}
}

// GetProof returns the MerkleProof for a given Account
func (s *StateDB) GetProof(a common.Address) ([][]byte, error) {
	var proof proofList
	err := s.trie.Prove(crypto.Keccak256(a.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetProof returns the StorageProof for given key
func (s *StateDB) GetStorageProof(a common.Address, key common.Hash) ([][]byte, error) {
	var proof proofList
	trie := s.StorageTrie(a)
	if trie == nil {
		return proof, errors.New("storage trie for requested address does not exist")
	}
	err := trie.Prove(crypto.Keccak256(key.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(s.db, hash)
	}
	return common.Hash{}
}

// Database retrieves the low level database supporting the lower level trie ops.
func (s *StateDB) Database() Database {
	return s.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (s *StateDB) StorageTrie(addr common.Address) Trie {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(s)
	return cpy.updateTrie(s.db)
}

func (s *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
func (s *StateDB) AddBalance(addr common.Address, amount *big.Int, tokenID uint64) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount, tokenID)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (s *StateDB) SubBalance(addr common.Address, amount *big.Int, tokenID uint64) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount, tokenID)
	}
}

func (s *StateDB) SetBalance(addr common.Address, amount *big.Int, tokenID uint64) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount, tokenID)
	}
}

func (s *StateDB) SetNonce(addr common.Address, nonce uint64) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

func (s *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (s *StateDB) SetState(addr common.Address, key, value common.Hash) {
	stateObject := s.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(s.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (s *StateDB) Suicide(addr common.Address) bool {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	s.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: stateObject.data.TokenBalances.GetBalanceMap(),
	})
	stateObject.markSuicided()
	stateObject.data.TokenBalances, _ = types.NewTokenBalances([]byte{})

	return true
}

//
// Setting, updating & deleting state object methods.
//

// updateStateObject writes the given object to the trie.
func (s *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	var mockAccount MockAccount
	if s.useMock {
		mockAccount = AccountToMock(stateObject.data)
		data, err = rlp.EncodeToBytes(mockAccount)
		if err != nil {
			panic(err)
		}
	}
	s.setError(s.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (s *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	s.setError(s.trie.TryDelete(addr[:]))
}

// Retrieve a state object given by the address. Returns nil if not found.
func (s *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := s.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := s.trie.TryGet(addr[:])
	if len(enc) == 0 {
		s.setError(err)
		return nil
	}
	var data Account
	if s.useMock {
		var mockAccount MockAccount
		if err := rlp.DecodeBytes(enc, &mockAccount); err != nil {
			log.Error("Failed to decode state object", "addr", addr, "err", err)
			return nil
		}
		rawTokenBalanes := types.NewEmptyTokenBalances()
		rawTokenBalanes.SetValue(mockAccount.Balance, qkcCommon.TokenIDEncode("QKC"))
		fullShardKey := types.Uint32(s.fullShardKey)
		data = Account{
			Nonce:         mockAccount.Nonce,
			TokenBalances: rawTokenBalanes,
			Root:          mockAccount.Root,
			CodeHash:      mockAccount.CodeHash,
			FullShardKey:  &fullShardKey,
		}
	} else {
		if err := rlp.DecodeBytes(enc, &data); err != nil {
			log.Error("Failed to decode state object", "addr", addr, "err", err)
			return nil
		}
	}

	// Insert into the live set.
	obj := newObject(s, addr, data)
	s.setStateObject(obj)
	return obj
}

func (s *StateDB) setStateObject(object *stateObject) {
	s.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil.
func (s *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := s.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = s.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (s *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = s.getStateObject(addr)
	newobj = newObject(s, addr, Account{})
	newobj.setNonce(0) // sets the object to dirty
	newobj.SetFullShardKey(s.fullShardKey)
	if prev == nil {
		s.journal.append(createObjectChange{account: &addr})
	} else {
		s.journal.append(resetObjectChange{prev: prev})
	}
	s.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
//   1. sends funds to sha(account ++ (nonce + 1))
//   2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (s *StateDB) CreateAccount(addr common.Address) {
	new, prev := s.createObject(addr)
	if prev != nil {
		new.setBalances(prev.data.TokenBalances.GetBalanceMap())
	}
}

func (s *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) {
	so := s.getStateObject(addr)
	if so == nil {
		return
	}
	it := trie.NewIterator(so.getTrie(s.db).NodeIterator(nil))
	for it.Next() {
		key := common.BytesToHash(s.trie.GetKey(it.Key))
		if value, dirty := so.dirtyStorage[key]; dirty {
			cb(key, value)
			continue
		}
		cb(key, common.BytesToHash(it.Value))
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (s *StateDB) Copy() *StateDB {
	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                s.db,
		trie:              s.db.CopyTrie(s.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(s.journal.dirties)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(s.journal.dirties)),
		refund:            s.refund,
		logs:              make(map[common.Hash][]*types.Log, len(s.logs)),
		logSize:           s.logSize,
		preimages:         make(map[common.Hash][]byte),
		journal:           newJournal(),
		senderDisallowMap: make(map[qkcaccount.Recipient]*big.Int, len(s.senderDisallowMap)),
	}
	// Copy the dirty states, logs, and preimages
	for addr := range s.journal.dirties {
		// As documented [here](https://github.com/ethereum/go-ethereum/pull/16485#issuecomment-380438527),
		// and in the Finalise-method, there is a case where an object is in the journal but not
		// in the stateObjects: OOG after touch on ripeMD prior to Byzantium. Thus, we need to check for
		// nil
		if object, exist := s.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range s.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = s.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	for hash, logs := range s.logs {
		cpy := make([]*types.Log, len(logs))
		for i, l := range logs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		state.logs[hash] = cpy
	}
	for hash, preimage := range s.preimages {
		state.preimages[hash] = preimage
	}
	for k, v := range s.senderDisallowMap {
		state.senderDisallowMap[k] = v
	}
	state.SetTimeStamp(s.timeStamp)
	state.SetBlockNumber(s.blockNumber)
	state.SetGasLimit(s.gasLimit)
	state.SetQuarkChainConfig(s.GetQuarkChainConfig())
	state.SetBlockCoinbase(s.blockCoinbase)
	return state
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateDB) Snapshot() int {
	id := s.nextRevisionId
	s.nextRevisionId++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (s *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.validRevisions), func(i int) bool {
		return s.validRevisions[i].id >= revid
	})
	if idx == len(s.validRevisions) || s.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	s.journal.revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (s *StateDB) GetRefund() uint64 {
	return s.refund
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.journal.dirties {
		stateObject, exist := s.stateObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the EVM emits new state logs.
func (s *StateDB) Prepare(thash, bhash common.Hash, ti int) {
	s.thash = thash
	s.bhash = bhash
	s.txIndex = ti
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = newJournal()
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}
func (s *StateDB) GetXShardReceiveGasUsed() *big.Int {
	if s.xShardReceiveGasUsed == nil {
		s.xShardReceiveGasUsed = new(big.Int)
	}
	return new(big.Int).Set(s.xShardReceiveGasUsed)
}

func (s *StateDB) SetXShardReceiveGasUsed(xShardReceivedGasUsed *big.Int) {
	s.xShardReceiveGasUsed = xShardReceivedGasUsed
}

func (s *StateDB) AppendXShardList(crossShardTxDeposit *types.CrossShardTransactionDeposit) {
	if s.xShardList == nil {
		s.xShardList = make([]*types.CrossShardTransactionDeposit, 0)
	}
	s.xShardList = append(s.xShardList, crossShardTxDeposit)
}

func (s *StateDB) GetXShardList() []*types.CrossShardTransactionDeposit {
	if s.xShardList == nil {
		s.xShardList = make([]*types.CrossShardTransactionDeposit, 0)
	}
	return s.xShardList
}
func (s *StateDB) SetFullShardKey(fullShardKey uint32) {
	s.fullShardKey = fullShardKey
}

func (s *StateDB) GetFullShardKey(addr common.Address) uint32 {
	stateObject := s.GetOrNewStateObject(addr)
	return stateObject.FullShardKey()

}

func (s *StateDB) AddBlockFee(fee map[uint64]*big.Int) {
	if s.blockFee == nil {
		s.blockFee = fee
		return
	}
	for k, v := range fee {
		preBalance, ok := s.blockFee[k]
		if !ok {
			preBalance = new(big.Int)
		}
		s.blockFee[k] = new(big.Int).Add(v, preBalance)
	}
}

func (s *StateDB) GetBlockFee() map[uint64]*big.Int {
	if s.blockFee == nil {
		return make(map[uint64]*big.Int)
	}
	return s.blockFee
}
func (s *StateDB) GetQuarkChainConfig() *config.QuarkChainConfig {
	return s.quarkChainConfig
}

func (s *StateDB) SetQuarkChainConfig(config *config.QuarkChainConfig) {
	s.quarkChainConfig = config
}
func (s *StateDB) GetGasUsed() *big.Int {
	if s.gasUsed == nil {
		s.gasUsed = new(big.Int)
	}
	return new(big.Int).Set(s.gasUsed)
}

func (s *StateDB) AddGasUsed(gasUsed *big.Int) {
	if s.gasUsed == nil {
		s.gasUsed = new(big.Int)
	}
	s.gasUsed.Add(s.gasUsed, gasUsed)
}

func (s *StateDB) SetGasUsed(gasUsed *big.Int) {
	s.gasUsed = gasUsed
}
func (s *StateDB) GetGasLimit() *big.Int {
	if s.gasLimit == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(s.gasLimit)
}

func (s *StateDB) SetGasLimit(gasLimit *big.Int) {
	s.gasLimit = gasLimit
}

func (s *StateDB) GetShardConfig() *config.ShardConfig {
	return s.shardConfig
}

func (s *StateDB) SetShardConfig(config *config.ShardConfig) {
	s.shardConfig = config
}

func (s *StateDB) SetSenderDisallowMap(senderDisallowMap map[qkcaccount.Recipient]*big.Int) {
	s.senderDisallowMap = senderDisallowMap
}
func (s *StateDB) GetSenderDisallowMap() map[qkcaccount.Recipient]*big.Int {
	return s.senderDisallowMap
}

func (s *StateDB) GetBlockCoinbase() qkcaccount.Recipient {
	return s.blockCoinbase
}

func (s *StateDB) SetBlockCoinbase(blockCoinbase qkcaccount.Recipient) {
	s.blockCoinbase = blockCoinbase
}

func (s *StateDB) SetTimeStamp(time uint64) {
	s.timeStamp = time
}

func (s *StateDB) GetTimeStamp() uint64 {
	return s.timeStamp
}

func (s *StateDB) SetBlockNumber(blockNumber uint64) {
	s.blockNumber = blockNumber
}

func (s *StateDB) GetBlockNumber() uint64 {
	return s.blockNumber
}

func (s *StateDB) SetTxCursorInfo(info *types.XShardTxCursorInfo) {
	s.xShardTxCursorInfo = info
}

func (s *StateDB) GetTxCursorInfo() *types.XShardTxCursorInfo {
	return s.xShardTxCursorInfo
}

func (s *StateDB) SetMockFlag(flag bool) {
	s.useMock = flag
}
