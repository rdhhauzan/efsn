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

package state

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/FusionFoundation/efsn/common"
	"github.com/FusionFoundation/efsn/crypto"
	"github.com/FusionFoundation/efsn/rlp"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (c Code) String() string {
	return string(c) //strings.Join(Disassemble(s), " ")
}

type Storage map[common.Hash]common.Hash

func (s Storage) String() (str string) {
	for key, value := range s {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}

	return
}

func (s Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range s {
		cpy[key] = value
	}

	return cpy
}

// stateObject represents an Ethereum account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	originStorage  Storage // Storage cache of original entries to dedup rewrites, reset for every transaction
	pendingStorage Storage // Storage entries that need to be flushed to disk, at the end of an entire block
	dirtyStorage   Storage // Storage entries that need to be flushed to disk
	fakeStorage    Storage // Fake storage which constructed by caller for debugging purpose.

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	if s.data.Nonce != 0 {
		return false
	}
	if len(s.data.BalancesVal) > 0 {
		return false
	}
	if len(s.data.TimeLockBalancesVal) > 0 {
		return false
	}
	if bytes.Equal(s.data.CodeHash, emptyCodeHash) == false {
		return false
	}
	if s.address.IsSpecialKeyAddress() == true {
		return false
	}
	return true
}

func (s *stateObject) deepCopyBalancesHash() []common.Hash {
	ret := make([]common.Hash, 0)
	if len(s.data.BalancesHash) == 0 {
		return ret
	}

	for _, v := range s.data.BalancesHash {
		ret = append(ret, v)
	}

	return ret
}

func (s *stateObject) deepCopyBalancesVal() []*big.Int {
	ret := make([]*big.Int, 0)
	if len(s.data.BalancesVal) == 0 {
		return ret
	}

	for _, v := range s.data.BalancesVal {
		a := new(big.Int).SetBytes(v.Bytes())
		ret = append(ret, a)
	}

	return ret
}

func (s *stateObject) deepCopyTimeLockBalancesHash() []common.Hash {
	ret := make([]common.Hash, 0)
	if len(s.data.TimeLockBalancesHash) == 0 {
		return ret
	}

	for _, v := range s.data.TimeLockBalancesHash {
		ret = append(ret, v)
	}

	return ret
}

func (s *stateObject) deepCopyTimeLockBalancesVal() []*common.TimeLock {
	ret := make([]*common.TimeLock, 0)
	if len(s.data.TimeLockBalancesVal) == 0 {
		return ret
	}

	for _, v := range s.data.TimeLockBalancesVal {
		t := v.Clone()
		ret = append(ret, t)
	}

	return ret
}

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	Nonce   uint64
	Notaion uint64

	// Balances         map[common.Hash]*big.Int
	// TimeLockBalances map[common.Hash]*common.TimeLock

	BalancesHash []common.Hash
	BalancesVal  []*big.Int

	TimeLockBalancesHash []common.Hash
	TimeLockBalancesVal  []*common.TimeLock

	Root     common.Hash // merkle root of the storage trie
	CodeHash []byte
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data Account) *stateObject {
	// if data.Balances == nil {
	// 	data.Balances = make(map[common.Hash]*big.Int)
	// }
	// if data.TimeLockBalances == nil {
	// 	data.TimeLockBalances = make(map[common.Hash]*common.TimeLock)
	// }

	if data.BalancesHash == nil {
		data.BalancesHash = make([]common.Hash, 0)
	}
	if data.BalancesVal == nil {
		data.BalancesVal = make([]*big.Int, 0)
	}
	if data.TimeLockBalancesHash == nil {
		data.TimeLockBalancesHash = make([]common.Hash, 0)
	}
	if data.TimeLockBalancesVal == nil {
		data.TimeLockBalancesVal = make([]*common.TimeLock, 0)
	}

	if data.CodeHash == nil {
		data.CodeHash = emptyCodeHash
	}
	if data.Root == (common.Hash{}) {
		data.Root = emptyRoot
	}
	return &stateObject{
		db:             db,
		address:        address,
		addrHash:       crypto.Keccak256Hash(address[:]),
		data:           data,
		originStorage:  make(Storage),
		pendingStorage: make(Storage),
		dirtyStorage:   make(Storage),
	}
}

// EncodeRLP implements rlp.Encoder.
func (s *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, s.data)
}

// setError remembers the first non-nil error it is called with.
func (s *stateObject) setError(err error) {
	if s.dbErr == nil {
		s.dbErr = err
	}
}

func (s *stateObject) markSuicided() {
	s.suicided = true
}

func (s *stateObject) touch() {
	s.db.journal.append(touchChange{
		account: &s.address,
	})
	if s.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		s.db.journal.dirty(s.address)
	}
}

func (s *stateObject) getTrie(db Database) Trie {
	if s.trie == nil {
		var err error
		s.trie, err = db.OpenStorageTrie(s.addrHash, s.data.Root)
		if err != nil {
			s.trie, _ = db.OpenStorageTrie(s.addrHash, common.Hash{})
			s.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return s.trie
}

// GetState returns a value in account storage.
// GetState retrieves a value from the account storage trie.
func (s *stateObject) GetState(db Database, key common.Hash) common.Hash {
	// If the fake storage is set, only lookup the state here(in the debugging mode)
	if s.fakeStorage != nil {
		return s.fakeStorage[key]
	}
	// If we have a dirty value for this state entry, return it
	value, dirty := s.dirtyStorage[key]
	if dirty {
		return value
	}
	// Otherwise return the entry's original value
	return s.GetCommittedState(db, key)
}

// GetCommittedState retrieves a value from the committed account storage trie.
func (s *stateObject) GetCommittedState(db Database, key common.Hash) common.Hash {
	// If the fake storage is set, only lookup the state here(in the debugging mode)
	if s.fakeStorage != nil {
		return s.fakeStorage[key]
	}
	// If we have a pending write or clean cached, return that
	if value, pending := s.pendingStorage[key]; pending {
		return value
	}
	if value, cached := s.originStorage[key]; cached {
		return value
	}
	// Otherwise load the value from the database
	enc, err := s.getTrie(db).TryGet(key[:])
	if err != nil {
		s.setError(err)
		return common.Hash{}
	}
	var value common.Hash
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			s.setError(err)
		}
		value.SetBytes(content)
	}
	s.originStorage[key] = value
	return value
}

// SetState updates a value in account storage.
func (s *stateObject) SetState(db Database, key, value common.Hash) {
	// If the fake storage is set, put the temporary state update here.
	if s.fakeStorage != nil {
		s.fakeStorage[key] = value
		return
	}
	// If the new value is the same as old, don't set
	prev := s.GetState(db, key)
	if prev == value {
		return
	}
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:  &s.address,
		key:      key,
		prevalue: prev,
	})
	s.setState(key, value)
}

// SetStorage replaces the entire state storage with the given one.
//
// After this function is called, all original state will be ignored and state
// lookup only happens in the fake state storage.
//
// Note this function should only be used for debugging purpose.
func (s *stateObject) SetStorage(storage map[common.Hash]common.Hash) {
	// Allocate fake storage if it's nil.
	if s.fakeStorage == nil {
		s.fakeStorage = make(Storage)
	}
	for key, value := range storage {
		s.fakeStorage[key] = value
	}
	// Don't bother journal since this function should only be used for
	// debugging and the `fake` storage won't be committed to database.
}

func (s *stateObject) setState(key, value common.Hash) {
	s.dirtyStorage[key] = value
}

// finalise moves all dirty storage slots into the pending area to be hashed or
// committed later. It is invoked at the end of every transaction.
func (s *stateObject) finalise() {
	for key, value := range s.dirtyStorage {
		s.pendingStorage[key] = value
	}
	if len(s.dirtyStorage) > 0 {
		s.dirtyStorage = make(Storage)
	}
}

// updateTrie writes cached storage modifications into the object's storage trie.
// It will return nil if the trie has not been loaded and no changes have been made
func (s *stateObject) updateTrie(db Database) Trie {
	// Make sure all dirty slots are finalized into the pending storage area
	s.finalise() // Don't prefetch any more, pull directly if need be
	if len(s.pendingStorage) == 0 {
		return s.trie
	}
	// Insert all the pending updates into the trie
	tr := s.getTrie(db)
	for key, value := range s.pendingStorage {
		// Skip noop changes, persist actual changes
		if value == s.originStorage[key] {
			continue
		}
		s.originStorage[key] = value

		if (value == common.Hash{}) {
			s.setError(tr.TryDelete(key[:]))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(bytes.TrimLeft(value[:], "\x00"))
		s.setError(tr.TryUpdate(key[:], v))
	}
	if len(s.pendingStorage) > 0 {
		s.pendingStorage = make(Storage)
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (s *stateObject) updateRoot(db Database) {
	// If nothing changed, don't bother with hashing anything
	if s.updateTrie(db) == nil {
		return
	}
	s.data.Root = s.trie.Hash()
}

// CommitTrie the storage trie of the object to db.
// This updates the trie root.
func (s *stateObject) CommitTrie(db Database) error {
	// If nothing changed, don't bother with hashing anything
	if s.updateTrie(db) == nil {
		return nil
	}
	if s.dbErr != nil {
		return s.dbErr
	}
	root, err := s.trie.Commit(nil)
	if err == nil {
		s.data.Root = root
	}
	return err
}

func (s *stateObject) balanceAssetIndex(assetID common.Hash) int {

	for i, v := range s.data.BalancesHash {
		if v == assetID {
			return i
		}
	}

	s.data.BalancesHash = append(s.data.BalancesHash, assetID)
	s.data.BalancesVal = append(s.data.BalancesVal, new(big.Int))

	return len(s.data.BalancesVal) - 1
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (s *stateObject) AddBalance(assetID common.Hash, amount *big.Int) {
	// EIP158: We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if s.empty() {
			s.touch()
		}
		return
	}
	index := s.balanceAssetIndex(assetID)

	s.SetBalance(assetID, new(big.Int).Add(s.data.BalancesVal[index], amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (s *stateObject) SubBalance(assetID common.Hash, amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	index := s.balanceAssetIndex(assetID)
	s.SetBalance(assetID, new(big.Int).Sub(s.data.BalancesVal[index], amount))
}

func (s *stateObject) SetBalance(assetID common.Hash, amount *big.Int) {
	index := s.balanceAssetIndex(assetID)

	s.db.journal.append(balanceChange{
		account: &s.address,
		assetID: assetID,
		prev:    new(big.Int).Set(s.data.BalancesVal[index]),
	})
	s.setBalance(assetID, amount)
}

func (s *stateObject) setBalance(assetID common.Hash, amount *big.Int) {
	index := s.balanceAssetIndex(assetID)
	s.data.BalancesVal[index] = amount
}

func (s *stateObject) timeLockAssetIndex(assetID common.Hash) int {

	for i, v := range s.data.TimeLockBalancesHash {
		if v == assetID {
			return i
		}
	}

	s.data.TimeLockBalancesHash = append(s.data.TimeLockBalancesHash, assetID)
	s.data.TimeLockBalancesVal = append(s.data.TimeLockBalancesVal, new(common.TimeLock))

	return len(s.data.TimeLockBalancesVal) - 1
}

// AddTimeLockBalance wacom
func (s *stateObject) AddTimeLockBalance(assetID common.Hash, amount *common.TimeLock, blockNumber *big.Int, timestamp uint64) {
	if amount.IsEmpty() {
		if s.empty() {
			s.touch()
		}
		return
	}

	index := s.timeLockAssetIndex(assetID)
	res := s.data.TimeLockBalancesVal[index]
	res = new(common.TimeLock).Add(res, amount)
	if res != nil {
		res = res.ClearExpired(timestamp)
	}

	s.SetTimeLockBalance(assetID, res)
}

// SubTimeLockBalance wacom
func (s *stateObject) SubTimeLockBalance(assetID common.Hash, amount *common.TimeLock, blockNumber *big.Int, timestamp uint64) {
	if amount.IsEmpty() {
		return
	}

	index := s.timeLockAssetIndex(assetID)
	res := s.data.TimeLockBalancesVal[index]
	res = new(common.TimeLock).Sub(res, amount)
	if res != nil {
		res = res.ClearExpired(timestamp)
	}

	s.SetTimeLockBalance(assetID, res)
}

func (s *stateObject) SetTimeLockBalance(assetID common.Hash, amount *common.TimeLock) {
	index := s.timeLockAssetIndex(assetID)

	s.db.journal.append(timeLockBalanceChange{
		account: &s.address,
		assetID: assetID,
		prev:    new(common.TimeLock).Set(s.data.TimeLockBalancesVal[index]),
	})
	s.setTimeLockBalance(assetID, amount)
}

func (s *stateObject) setTimeLockBalance(assetID common.Hash, amount *common.TimeLock) {
	index := s.timeLockAssetIndex(assetID)
	s.data.TimeLockBalancesVal[index] = amount
}

// Return the gas back to the origin. Used by the Virtual machine or Closures
func (s *stateObject) ReturnGas(gas *big.Int) {}

func (s *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, s.address, s.data)
	if s.trie != nil {
		stateObject.trie = db.db.CopyTrie(s.trie)
	}
	stateObject.code = s.code
	stateObject.dirtyStorage = s.dirtyStorage.Copy()
	stateObject.originStorage = s.originStorage.Copy()
	stateObject.pendingStorage = s.pendingStorage.Copy()
	stateObject.suicided = s.suicided
	stateObject.dirtyCode = s.dirtyCode
	stateObject.deleted = s.deleted
	stateObject.data.BalancesHash = s.deepCopyBalancesHash()
	stateObject.data.BalancesVal = s.deepCopyBalancesVal()
	stateObject.data.TimeLockBalancesHash = s.deepCopyTimeLockBalancesHash()
	stateObject.data.TimeLockBalancesVal = s.deepCopyTimeLockBalancesVal()
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (s *stateObject) Address() common.Address {
	return s.address
}

// Code returns the contract code associated with this object, if any.
func (s *stateObject) Code(db Database) []byte {
	if s.code != nil {
		return s.code
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(s.addrHash, common.BytesToHash(s.CodeHash()))
	if err != nil {
		s.setError(fmt.Errorf("can't load code hash %x: %v", s.CodeHash(), err))
	}
	s.code = code
	return code
}

// CodeSize returns the size of the contract code associated with this object,
// or zero if none. This method is an almost mirror of Code, but uses a cache
// inside the database to avoid loading codes seen recently.
func (s *stateObject) CodeSize(db Database) int {
	if s.code != nil {
		return len(s.code)
	}
	if bytes.Equal(s.CodeHash(), emptyCodeHash) {
		return 0
	}
	size, err := db.ContractCodeSize(s.addrHash, common.BytesToHash(s.CodeHash()))
	if err != nil {
		s.setError(fmt.Errorf("can't load code size %x: %v", s.CodeHash(), err))
	}
	return size
}

func (s *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := s.Code(s.db.db)
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash(),
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *stateObject) setCode(codeHash common.Hash, code []byte) {
	s.code = code
	s.data.CodeHash = codeHash[:]
	s.dirtyCode = true
}

func (s *stateObject) SetNonce(nonce uint64) {
	s.db.journal.append(nonceChange{
		account: &s.address,
		prev:    s.data.Nonce,
	})
	s.setNonce(nonce)
}

func (s *stateObject) setNonce(nonce uint64) {
	s.data.Nonce = nonce
}

func (s *stateObject) SetNotation(notation uint64) {
	s.db.journal.append(notationChange{
		account: &s.address,
		prev:    s.data.Notaion,
	})
	s.setNotation(notation)
}

func (s *stateObject) setNotation(notation uint64) {
	s.data.Notaion = notation
}

func (s *stateObject) CodeHash() []byte {
	return s.data.CodeHash
}

func (s *stateObject) CopyBalances() map[common.Hash]string {
	retBalances := make(map[common.Hash]string)
	for i, v := range s.data.BalancesHash {
		if s.data.BalancesVal[i].Sign() != 0 {
			retBalances[v] = s.data.BalancesVal[i].String()
		}
	}
	return retBalances
}

func (s *stateObject) Balance(assetID common.Hash) *big.Int {
	index := s.balanceAssetIndex(assetID)
	return s.data.BalancesVal[index]
}

func (s *stateObject) CopyTimeLockBalances() map[common.Hash]*common.TimeLock {
	retBalances := make(map[common.Hash]*common.TimeLock)
	for i, v := range s.data.TimeLockBalancesHash {
		if !s.data.TimeLockBalancesVal[i].IsEmpty() {
			retBalances[v] = s.data.TimeLockBalancesVal[i]
		}
	}
	return retBalances
}

func (s *stateObject) TimeLockBalance(assetID common.Hash) *common.TimeLock {
	index := s.timeLockAssetIndex(assetID)
	return s.data.TimeLockBalancesVal[index]
}

func (s *stateObject) Nonce() uint64 {
	return s.data.Nonce
}

func (s *stateObject) Notation() uint64 {
	return s.data.Notaion
}

// Never called, but must be present to allow stateObject to be used
// as a vm.Account interface that also satisfies the vm.ContractRef
// interface. Interfaces are awesome.
func (s *stateObject) Value() *big.Int {
	panic("Value on stateObject should never be called")
}
