pragma solidity ^0.5.4;

import "https://github.com/cross-chain/openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";
import "https://github.com/cross-chain/openzeppelin-contracts/contracts/token/ERC20/ERC20Detailed.sol";
import "https://github.com/cross-chain/efsn/contracts/timelock/TimeLockContract.sol";

contract TimeLockERC20 is ERC20, ERC20Detailed, TimeLockContract {
    event AssetToTimeLockEvent(address from, address to, uint value, uint start, uint end);
    event TimeLockToTimeLockEvent(address from, address to, uint value, uint start, uint end);
    event TimeLockToAssetEvent(address from, address to, uint value);

    constructor() public ERC20Detailed("TimeLockERC20 Test Token", "TEST", 18) {
        _mint(msg.sender, 50000000*10**18);
    }

    function getTimeLockBalance(address addr) public view returns (string memory) {
        (, bytes memory data) = _getTimeLockBalance(addr, false);
        return string(data);
    }

    function getRawTimeLockBalance(address addr) public view returns (string memory) {
        (, bytes memory data) = _getTimeLockBalance(addr, true);
        return string(data);
    }

    function hasTimeLockBalance(address addr, uint256 value, uint64 start, uint64 end) public view returns (bool) {
        return _hasTimeLockBalance(addr, value, start, end);
    }

    function getTimeLockValue(address addr, uint64 start, uint64 end) public view returns (uint256 value) {
        return _getTimeLockValue(addr, start, end);
    }

    function assetToTimeLock(address to, uint256 value, uint64 start, uint64 end) public returns (bool success) {
        require(value > 0, "value is zero");
        (start, end) = adjustStartEndTime(start, end);
        _subBalance(msg.sender, value);
        (success,) = _addTimeLockBalance(to, value, start, end);
        require(success);
        if (start > block.timestamp) {
            (success,) = _addTimeLockBalance(msg.sender, value, uint64(block.timestamp), start-1);
            require(success);
        }
        if (end < uint64(-1)) {
            (success,) = _addTimeLockBalance(msg.sender, value, end+1, uint64(-1));
            require(success);
        }
        emit AssetToTimeLockEvent(msg.sender, to, value, start, end);
        return true;
    }

    function timeLockToTimeLock(address to, uint256 value, uint64 start, uint64 end) public returns (bool success) {
        require(value > 0, "value is zero");
        (start, end) = adjustStartEndTime(start, end);
        (success,) = _subTimeLockBalance(msg.sender, value, start, end);
        require(success);
        (success,) = _addTimeLockBalance(to, value, start, end);
        require(success);
        emit TimeLockToTimeLockEvent(msg.sender, to, value, start, end);
        return true;
    }

    function timeLockToAsset(address to, uint256 value) public returns (bool success) {
        require(value > 0, "value is zero");
        (success,) = _subTimeLockBalance(msg.sender, value, uint64(block.timestamp), uint64(-1));
        require(success);
        _addBalance(to, value);
        emit TimeLockToAssetEvent(msg.sender, to, value);
        return true;
    }

    function adjustStartEndTime(uint64 startTime, uint64 endTime) internal view returns (uint64, uint64) {
        uint64 timestamp = uint64(block.timestamp);
        if (startTime < timestamp) {
            startTime = timestamp;
        }
        if (endTime == 0) {
            endTime = uint64(-1);
        }
        return (startTime, endTime);
    }
}
