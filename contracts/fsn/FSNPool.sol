pragma solidity ^0.5.4;

import "https://github.com/cross-chain/efsn/contracts/fsn/FSNContract.sol";
import "https://github.com/cross-chain/efsn/contracts/timelock/TimeLockContract.sol";

contract FSNPool is FSNContract, TimeLockContract {
    event LogDeposit(address indexed _from, uint256 _value, uint64 _start, uint64 _end);
    event LogWithdraw(address indexed _from, uint256 _value, uint64 _start, uint64 _end);

    bytes32 constant public FSNPoolAsset = 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff;

    address public owner;
    modifier onlyOwner {
        require(msg.sender == owner, "only owner");
        _;
    }

    constructor() public {
        owner = msg.sender;
    }

    function receiveAsset(bytes32 assetID, uint64 startTime, uint64 endTime, SendAssetFlag flag, uint256[] memory extraInfo) payable public returns (bool success) {
        (flag, extraInfo); // silence warning of Unused function parameter
        require(msg.value > 0, "value is zero");
        require(assetID == FSNPoolAsset, "wrong asset");
        (startTime, endTime) = adjustStartEndTime(startTime, endTime);
        _addTimeLockBalance(msg.sender, msg.value, startTime, endTime);
        emit LogDeposit(msg.sender, msg.value, startTime, endTime);
        return true;
    }

    function sendAsset(bytes32 asset, address to, uint256 value, uint64 start, uint64 end, SendAssetFlag flag) onlyOwner public returns (bool success) {
        require(value > 0, "value is zero");
        (start, end) = adjustStartEndTime(start, end);
        (success,) = _sendAsset(asset, to, value, start, end, flag);
        require(success, "call sendAsset failed");
        return true;
    }

    function withdraw(uint256 value, uint64 start, uint64 end) public returns (bool success) {
        require(value > 0, "value is zero");
        (start, end) = adjustStartEndTime(start, end);
        (success,) = _subTimeLockBalance(msg.sender, value, start, end);
        require(success, "call subTimeLockBalance failed");
        (success,) = _sendAsset(FSNPoolAsset, msg.sender, value, start, end, SendAssetFlag.UseAny);
        require(success, "call sendAsset failed");
        emit LogWithdraw(msg.sender, value, start, end);
        return true;
    }

    function getTimeLockBalance(address addr) public view returns (string memory balance) {
        (, bytes memory data) = _getTimeLockBalance(addr, false);
        return string(data);
    }

    function getRawTimeLockBalance(address addr) public view returns (string memory balance) {
        (, bytes memory data) = _getTimeLockBalance(addr, true);
        return string(data);
    }

    function getTimeLockValue(address addr, uint64 start, uint64 end) public view returns (uint256 value) {
        return _getTimeLockValue(addr, start, end);
    }

    function adjustStartEndTime(uint64 startTime, uint64 endTime) internal view returns (uint64, uint64) {
        uint64 timestamp = uint64(block.timestamp);
        if (startTime < timestamp) {
            startTime = timestamp;
        }
        if (endTime == 0) {
            endTime = uint64(-1);
        }
        require(startTime <= endTime, "wrong time range");
        return (startTime, endTime);
    }
}
