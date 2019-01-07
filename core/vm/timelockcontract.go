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

	ErrUnknownFunc      = errors.New("unknown func type")
	ErrNotEnoughBalance = errors.New("not enough balance")
	ErrWrongTimeRange   = errors.New("wrong time range")
	ErrValueOverflow    = errors.New("value overflow")
	ErrWrongLenOfInput  = errors.New("wrong length of input")
)

type TimeLockFuncType uint64

const (
	FuncUnknownTimeLockFunc TimeLockFuncType = iota
	FuncGetTimeLockBalance                   // 1
	FuncHasTimeLockBalance                   // 2
	FuncGetTimeLockValue                     // 3
	FuncAddTimeLockBalance                   // 4
	FuncSubTimeLockBalance                   // 5
)

func (f TimeLockFuncType) Name() string {
	switch f {
	case FuncGetTimeLockBalance:
		return "getTimeLockBalance"
	case FuncHasTimeLockBalance:
		return "hasTimeLockBalance"
	case FuncGetTimeLockValue:
		return "getTimeLockValue"
	case FuncAddTimeLockBalance:
		return "addTimeLockBalance"
	case FuncSubTimeLockBalance:
		return "subTimeLockBalance"
	}
	return "unknownTimeLockFunc"
}

type TimeLockContract struct {
	evm      *EVM
	contract *Contract
	input    []byte
}

func NewTimeLockContract(evm *EVM, contract *Contract) *TimeLockContract {
	return &TimeLockContract{
		evm:      evm,
		contract: contract,
	}
}

func (c *TimeLockContract) RequiredGas(input []byte) uint64 {
	return params.TimeLockCalcGas
}

func (c *TimeLockContract) Run(input []byte) (ret []byte, err error) {
	c.input = input
	err = ErrUnknownFunc
	funcType := FuncUnknownTimeLockFunc
	if len(c.input) >= 32 {
		funcType = TimeLockFuncType(c.getBigInt(0).Uint64())
		if _, err = c.contract.GetParentCaller(); err == nil {
			switch funcType {
			case FuncGetTimeLockBalance:
				ret, err = c.getTimeLockBalance()
			case FuncHasTimeLockBalance:
				ret, err = c.hasTimeLockBalance()
			case FuncGetTimeLockValue:
				ret, err = c.getTimeLockValue()
			case FuncAddTimeLockBalance:
				ret, err = c.addTimeLockBalance()
			case FuncSubTimeLockBalance:
				ret, err = c.subTimeLockBalance()
			}
		}
	}
	if err != nil {
		common.DebugInfo("Run TimeLockContract error",
			"func", funcType.Name(),
			"input", input,
			"err", err,
		)
		return toErrData(err), err
	}
	return ret, nil
}

func (c *TimeLockContract) getTimeLockBalance() ([]byte, error) {
	addr := common.BytesToAddress(getData(c.input, 32, 32))
	isRaw := c.getBigInt(64).Sign() != 0
	timelock := c._getTimeLockBalance(addr)
	if isRaw {
		return []byte(timelock.RawString()), nil
	}
	return []byte(timelock.String()), nil
}

func (c *TimeLockContract) hasTimeLockBalance() ([]byte, error) {
	param, err := c.parseParams(true)
	if err != nil {
		return nil, err
	}
	timelock := c._getTimeLockBalance(param.address)
	cmp := timelock.Cmp(param.getTimeLock())
	if cmp < 0 {
		return []byte{0x0}, nil
	}
	return []byte{0x1}, nil
}

func (c *TimeLockContract) getTimeLockValue() ([]byte, error) {
	param, err := c.parseParams(false)
	if err != nil {
		return nil, err
	}
	timelock := c._getTimeLockBalance(param.address)
	value := timelock.GetSpendableValue(param.start, param.end)
	return common.LeftPadBytes(value.Bytes(), 32), nil
}

func (c *TimeLockContract) addTimeLockBalance() ([]byte, error) {
	param, err := c.parseParams(true)
	if err != nil {
		return nil, err
	}
	timelock := param.getTimeLock()
	c._setTimeLockBalance(param.address, new(common.TimeLock).Add(
		c._getTimeLockBalance(param.address),
		timelock,
	))
	return toOKData("addTimeLockBalance"), nil
}

func (c *TimeLockContract) subTimeLockBalance() ([]byte, error) {
	param, err := c.parseParams(true)
	if err != nil {
		return nil, err
	}
	timelock := param.getTimeLock()
	timeLockBalance := c._getTimeLockBalance(param.address)
	if timeLockBalance.Cmp(timelock) < 0 {
		return nil, ErrNotEnoughBalance
	}
	c._setTimeLockBalance(param.address, new(common.TimeLock).Sub(
		timeLockBalance,
		timelock,
	))
	return toOKData("subTimeLockBalance"), nil
}

type TimeLockFuncParams struct {
	address common.Address
	value   *big.Int
	start   uint64
	end     uint64
}

func (p *TimeLockFuncParams) getTimeLock() *common.TimeLock {
	return common.GetTimeLock(p.value, p.start, p.end)
}

func (c *TimeLockContract) parseParams(hasValue bool) (*TimeLockFuncParams, error) {
	var param TimeLockFuncParams
	var overflow bool

	pos := uint64(32)
	param.address = common.BytesToAddress(getData(c.input, pos, 32))
	pos += 32
	if hasValue {
		param.value = c.getBigInt(pos)
		pos += 32
	}
	if param.start, overflow = c.getUint64(pos); overflow {
		return nil, ErrValueOverflow
	}
	pos += 32
	if param.end, overflow = c.getUint64(pos); overflow {
		return nil, ErrValueOverflow
	}
	pos += 32

	if uint64(len(c.input)) != pos {
		return nil, ErrWrongLenOfInput
	}

	// adjust
	timestamp := c.evm.Context.Time.Uint64()
	if param.start < timestamp {
		param.start = timestamp
	}
	if param.end == 0 {
		param.end = common.TimeLockForever
	}

	// check
	if param.start > param.end {
		return nil, ErrWrongTimeRange
	}

	return &param, nil
}

func (c *TimeLockContract) getBigInt(pos uint64) *big.Int {
	return new(big.Int).SetBytes(getData(c.input, pos, 32))
}

func (c *TimeLockContract) getUint64(pos uint64) (uint64, bool) {
	return getUint64(c.input, pos, 32)
}

func (c *TimeLockContract) genKey(addr common.Address) []byte {
	caller := c.contract.Caller()
	key := make([]byte, 40)
	copy(key[0:20], caller.Bytes())
	copy(key[20:40], addr.Bytes())
	return key
}

func (c *TimeLockContract) _getTimeLockBalance(addr common.Address) *common.TimeLock {
	var timelock, empty common.TimeLock
	db := c.evm.StateDB
	key := c.genKey(addr)
	data := db.GetStructData(TimeLockContractAddress, key)
	if len(data) == 0 {
		return &empty
	}
	if err := rlp.DecodeBytes(data, &timelock); err != nil {
		return &empty
	}
	return &timelock
}

func (c *TimeLockContract) _setTimeLockBalance(addr common.Address, timelock *common.TimeLock) {
	data := make([]byte, 0)
	if timelock != nil {
		timestamp := c.evm.Context.Time.Uint64()
		timelock = timelock.ClearExpired(timestamp)
		if !timelock.IsEmpty() {
			data, _ = rlp.EncodeToBytes(timelock)
		}
	}
	db := c.evm.StateDB
	key := c.genKey(addr)
	db.SetStructData(TimeLockContractAddress, key, data)
}

func toOKData(str string) []byte {
	return []byte("Ok: " + str)
}

func toErrData(err error) []byte {
	return []byte("Error: " + err.Error())
}
