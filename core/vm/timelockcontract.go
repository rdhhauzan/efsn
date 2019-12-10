package vm

import (
	"errors"
	"math/big"

	"github.com/FusionFoundation/efsn/common"
	"github.com/FusionFoundation/efsn/params"
	"github.com/FusionFoundation/efsn/rlp"
)

var (
	TimeLockContractAddress = common.HexToAddress("0x8888888888888888888888888888888888888888")

	TLEnableTimeLockErr  = errors.New("can not enable time lock")
	TLTimeLockNotEnabled = errors.New("time lock not enabled")
	TLUnknownFunc        = errors.New("unknown func type")
	TLNoEnoughTimeLock   = errors.New("no enough time lock")
)

type TimeLockFuncType uint64

const (
	FuncUnknownTimeLockFunc   = iota
	FuncEnableTimeLock        // 1
	FuncGetTimeLockBalance    // 2
	FuncGetRawTimeLockBalance // 3
	FuncHasTimeLockBalance    // 4
	FuncAssetToTimeLock       // 5
	FuncTimeLockToTimeLock    // 6
	FuncTimeLockToAsset       // 7
)

func (f TimeLockFuncType) Name() string {
	switch f {
	case FuncEnableTimeLock:
		return "enableTimeLock"
	case FuncGetTimeLockBalance:
		return "getTimeLockBalance"
	case FuncGetRawTimeLockBalance:
		return "getRawTimeLockBalance"
	case FuncHasTimeLockBalance:
		return "hasTimeLockBalance"
	case FuncAssetToTimeLock:
		return "assetToTimeLock"
	case FuncTimeLockToTimeLock:
		return "timeLockToTimeLock"
	case FuncTimeLockToAsset:
		return "timeLockToAsset"
	}
	return "unknownTimeLockFunc"
}

type TimeLockContract struct {
	evm    *EVM
	caller common.Address
	input  []byte
}

func NewTimeLockContract(evm *EVM, caller common.Address) *TimeLockContract {
	return &TimeLockContract{
		evm:    evm,
		caller: caller,
	}
}

func (c *TimeLockContract) RequiredGas(input []byte) uint64 {
	return params.TimeLockCalcGas
}

func (c *TimeLockContract) Run(input []byte) (ret []byte, err error) {
	c.input = input
	err = TLUnknownFunc
	funcType := TimeLockFuncType(c.getBigInt(0).Uint64())
	if funcType != FuncEnableTimeLock && !c._isTimeLockEnabled(c.caller) {
		err = TLTimeLockNotEnabled
	} else if len(c.input) >= 32 {
		switch funcType {
		case FuncEnableTimeLock:
			ret, err = c.enableTimeLock()
		case FuncGetTimeLockBalance:
			ret, err = c.getTimeLockBalance()
		case FuncGetRawTimeLockBalance:
			ret, err = c.getRawTimeLockBalance()
		case FuncHasTimeLockBalance:
			ret, err = c.hasTimeLockBalance()
		case FuncAssetToTimeLock:
			ret, err = c.assetToTimeLock()
		case FuncTimeLockToTimeLock:
			ret, err = c.timeLockToTimeLock()
		case FuncTimeLockToAsset:
			ret, err = c.timeLockToAsset()
		}
	}
	if err != nil {
		common.DebugInfo("Run TimeLockContract error", "func", funcType.Name(), "input", input, "err", err)
		return toErrData(err), err
	}
	return ret, err
}

func toOKData(str string) []byte {
	return []byte("Ok: " + str)
}

func toErrData(err error) []byte {
	return []byte("Error: " + err.Error())
}

func (c *TimeLockContract) enableTimeLock() ([]byte, error) {
	if c.evm.StateDB.GetCode(c.caller) == nil {
		return nil, TLEnableTimeLockErr
	}
	c._enableTimeLock(c.caller)
	return toOKData("enableTimeLock"), nil
}

func (c *TimeLockContract) getTimeLockBalance() ([]byte, error) {
	addr := common.BytesToAddress(getData(c.input, 32, 32))
	return []byte(c._getTimeLockBalance(addr).String()), nil
}

func (c *TimeLockContract) getRawTimeLockBalance() ([]byte, error) {
	addr := common.BytesToAddress(getData(c.input, 32, 32))
	return []byte(c._getTimeLockBalance(addr).RawString()), nil
}

func (c *TimeLockContract) hasTimeLockBalance() ([]byte, error) {
	param, err := c.parseParams(32, false, true)
	if err != nil {
		return nil, err
	}
	cmp := c._getTimeLockBalance(param.from).Cmp(param.getTimeLock())
	if cmp < 0 {
		return []byte("false"), nil
	}
	return []byte("true"), nil
}

// pre-condition: sub asset from 'FROM'
func (c *TimeLockContract) assetToTimeLock() ([]byte, error) {
	param, err := c.parseParams(32, true, true)
	if err != nil {
		return nil, err
	}
	if param.from == param.to {
		c._setTimeLockBalance(param.timestamp, param.to, new(common.TimeLock).Add(
			c._getTimeLockBalance(param.from),
			param.getTotalTimeLock(),
		))
	} else {
		surplus := param.getSurplusTimeLock()
		if !surplus.IsEmpty() {
			c._setTimeLockBalance(param.timestamp, param.from, new(common.TimeLock).Add(
				c._getTimeLockBalance(param.from),
				surplus,
			))
		}
		c._setTimeLockBalance(param.timestamp, param.to, new(common.TimeLock).Add(
			c._getTimeLockBalance(param.to),
			param.getTimeLock(),
		))
	}
	return toOKData("assetToTimeLock"), nil
}

func (c *TimeLockContract) timeLockToTimeLock() ([]byte, error) {
	param, err := c.parseParams(32, true, true)
	if err != nil {
		return nil, err
	}
	needed := param.getTimeLock()
	fromTimeLockBalance := c._getTimeLockBalance(param.from)
	cmp := fromTimeLockBalance.Cmp(needed)
	if cmp < 0 {
		return nil, TLNoEnoughTimeLock
	}
	if param.from != param.to {
		c._setTimeLockBalance(param.timestamp, param.from, new(common.TimeLock).Sub(
			fromTimeLockBalance,
			needed,
		))
		c._setTimeLockBalance(param.timestamp, param.to, new(common.TimeLock).Add(
			c._getTimeLockBalance(param.to),
			needed,
		))
	}
	return toOKData("timeLockToTimeLock"), nil
}

// post-condition: add asset to 'TO'
func (c *TimeLockContract) timeLockToAsset() ([]byte, error) {
	param, err := c.parseParams(32, false, false)
	if err != nil {
		return nil, err
	}
	needed := param.getTimeLock()
	fromTimeLockBalance := c._getTimeLockBalance(param.from)
	cmp := fromTimeLockBalance.Cmp(needed)
	if cmp < 0 {
		return nil, TLNoEnoughTimeLock
	}
	c._setTimeLockBalance(param.timestamp, param.from, new(common.TimeLock).Sub(
		fromTimeLockBalance,
		needed,
	))
	return toOKData("timeLockToAsset"), nil
}

type TimeLockFuncParams struct {
	from, to              common.Address
	value                 *big.Int
	timestamp, start, end uint64
}

func (p *TimeLockFuncParams) getTimeLock() *common.TimeLock {
	return common.NewTimeLock(&common.TimeLockItem{
		Value:     p.value,
		StartTime: p.start,
		EndTime:   p.end,
	})
}

func (p *TimeLockFuncParams) getTotalTimeLock() *common.TimeLock {
	return common.NewTimeLock(&common.TimeLockItem{
		Value:     p.value,
		StartTime: p.timestamp,
		EndTime:   common.TimeLockForever,
	})
}

func (p *TimeLockFuncParams) getSurplusTimeLock() *common.TimeLock {
	left := common.NewTimeLock()
	if p.start > p.timestamp {
		left.Items = append(left.Items, &common.TimeLockItem{
			Value:     p.value,
			StartTime: p.timestamp,
			EndTime:   p.start - 1,
		})
	}
	if p.end < common.TimeLockForever {
		left.Items = append(left.Items, &common.TimeLockItem{
			Value:     p.value,
			StartTime: p.end + 1,
			EndTime:   common.TimeLockForever,
		})
	}
	return left
}

func (c *TimeLockContract) parseParams(pos uint64, hasToAddress, hasStartEndTime bool) (*TimeLockFuncParams, error) {
	var param TimeLockFuncParams

	param.timestamp = c.getBigInt(pos).Uint64()
	pos += 32

	param.from = common.BytesToAddress(getData(c.input, pos, 32))
	pos += 32

	if hasToAddress {
		param.to = common.BytesToAddress(getData(c.input, pos, 32))
		pos += 32
	}

	param.value = c.getBigInt(pos)
	pos += 32

	if hasStartEndTime {
		param.start = c.getBigInt(pos).Uint64()
		pos += 32
		param.end = c.getBigInt(pos).Uint64()
	}

	// adjust
	if param.start < param.timestamp {
		param.start = param.timestamp
	}
	if param.end == 0 {
		param.end = common.TimeLockForever
	}

	// check
	item := &common.TimeLockItem{
		StartTime: param.start,
		EndTime:   param.end,
		Value:     param.value,
	}
	if err := item.IsValid(); err != nil {
		common.DebugInfo("parseParams", "err", err)
		return nil, err
	}

	return &param, nil
}

func (c *TimeLockContract) getBigInt(pos uint64) *big.Int {
	return new(big.Int).SetBytes(getData(c.input, pos, 32))
}

func (c *TimeLockContract) _isTimeLockEnabled(addr common.Address) bool {
	db := c.evm.StateDB
	data := db.GetStructData(TimeLockContractAddress, addr.Bytes())
	return len(data) == 1 && data[0] == 1
}

func (c *TimeLockContract) _enableTimeLock(addr common.Address) {
	if c._isTimeLockEnabled(addr) {
		return
	}
	db := c.evm.StateDB
	db.SetStructData(TimeLockContractAddress, addr.Bytes(), []byte{1})
}

func (c *TimeLockContract) _getTimeLockBalance(addr common.Address) *common.TimeLock {
	var timelock, empty common.TimeLock
	db := c.evm.StateDB
	key := make([]byte, 2*common.AddressLength)
	copy(key[:common.AddressLength], c.caller.Bytes())
	copy(key[common.AddressLength:], addr.Bytes())
	data := db.GetStructData(TimeLockContractAddress, key)
	if len(data) == 0 {
		return &empty
	}
	if err := rlp.DecodeBytes(data, &timelock); err != nil {
		return &empty
	}
	return &timelock
}

func (c *TimeLockContract) _setTimeLockBalance(timestamp uint64, addr common.Address, timelock *common.TimeLock) {
	var data []byte
	if timelock != nil {
		timelock = timelock.ClearExpired(timestamp)
		if !timelock.IsEmpty() {
			data, _ = rlp.EncodeToBytes(timelock)
		}
	}
	db := c.evm.StateDB
	key := make([]byte, 2*common.AddressLength)
	copy(key[:common.AddressLength], c.caller.Bytes())
	copy(key[common.AddressLength:], addr.Bytes())
	db.SetStructData(TimeLockContractAddress, key, data)
}
