pragma solidity ^0.5.0;

contract TimeLockContract {
    // precompiled time lock contract address
    address constant timeLockCalc = address(0x8888888888888888888888888888888888888888);

    function _enableTimeLock() internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(1);
        return timeLockCalc.call(input);
    }

    function _getTimeLockBalance(address addr) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(2, addr);
        return timeLockCalc.call(input);
    }

    function _getRawTimeLockBalance(address addr) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(3, addr);
        return timeLockCalc.call(input);
    }

    function _hasTimeLockBalance(address addr, uint256 value, uint64 start, uint64 end) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(4, block.timestamp, addr, value, start, end);
        return timeLockCalc.call(input);
    }

    function _assetToTimeLock(address from, address to, uint256 value, uint64 start, uint64 end) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(5, block.timestamp, from, to, value, start, end);
        return timeLockCalc.call(input);
    }

    function _timeLockToTimeLock(address from, address to, uint256 value, uint64 start, uint64 end) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(6, block.timestamp, from, to, value, start, end);
        return timeLockCalc.call(input);
    }

    function _timeLockToAsset(address from, uint256 value) internal returns (bool, bytes memory) {
        bytes memory input = abi.encode(7, block.timestamp, from, value);
        return timeLockCalc.call(input);
    }
}
