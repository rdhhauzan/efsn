pragma solidity ^0.5.4;

contract TimeLockContract {
    address constant precompile = address(0x8888888888888888888888888888888888888888);

    function _getTimeLockBalance(address addr, bool isRaw) internal view returns (bool, bytes memory) {
        bytes memory input = abi.encode(1, addr, isRaw);
        return precompile.staticcall(input);
    }

    function _hasTimeLockBalance(address addr, uint256 value, uint64 start, uint64 end) internal view returns (bool) {
        bytes memory input = abi.encode(2, addr, value, start, end);
        (bool ok, bytes memory data) = precompile.staticcall(input);
        return ok && data[0] != 0;
    }

    function _getTimeLockValue(address addr, uint64 start, uint64 end) internal view returns (uint256 value) {
        bytes memory input = abi.encode(3, addr, start, end);
        (bool ok, bytes memory data) = precompile.staticcall(input);
        if (ok) {
            return _toUint256(data, 0);
        }
        return 0;
    }

    function _addTimeLockBalance(address to, uint256 value, uint64 start, uint64 end) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(4, to, value, start, end);
        return precompile.call(input);
    }

    function _subTimeLockBalance(address from, uint256 value, uint64 start, uint64 end) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(5, from, value, start, end);
        return precompile.call(input);
    }

    function _toUint256(bytes memory _bytes, uint _start) internal pure returns (uint256) {
        require(_bytes.length >= (_start + 32));
        uint256 tempUint;

        assembly {
            tempUint := mload(add(add(_bytes, 0x20), _start))
        }

        return tempUint;
    }
}
