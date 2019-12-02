pragma solidity ^0.5.0;

import "https://github.com/cross-chain/openzeppelin-contracts/contracts/token/ERC20/ERC20.sol";
import "https://github.com/cross-chain/efsn/contracts/timelock/TimeLockContract.sol";

contract TimeLockERC20 is ERC20, TimeLockContract {
    event AssetToTimeLockEvent(address from, address to, uint value, uint start, uint end);
    event TimeLockToTimeLockEvent(address from, address to, uint value, uint start, uint end);
    event TimeLockToAssetEvent(address from, address to, uint value);

    address private _creator;

    constructor() public {
        _creator = _msgSender();
    }

    function enableTimeLock() public returns (string memory) {
        if (_msgSender() != _creator) {
            return "Error: only contract creator can enable time lock";
        }
        (, bytes memory data) = _enableTimeLock();
        return string(data);
    }

    function getTimeLockBalance(address addr) public returns (string memory) {
        (, bytes memory data) = _getTimeLockBalance(addr);
        return string(data);
    }

    function getRawTimeLockBalance(address addr) public returns (string memory) {
        (, bytes memory data) = _getRawTimeLockBalance(addr);
        return string(data);
    }

    function hasTimeLockBalance(address addr, uint256 value, uint64 start, uint64 end) public returns (string memory) {
        (, bytes memory data) = _hasTimeLockBalance(addr, value, start, end);
        return string(data);
    }

    function assetToTimeLock(address to, uint256 value, uint64 start, uint64 end) public returns (string memory) {
        address from = _msgSender();
        uint256 balance = balanceOf(from);
        if (balance < value) {
            return "Error: no enough asset balance";
        }
        (bool ok, bytes memory data) = _assetToTimeLock(from, to, value, start, end);
        if (!ok) {
            return string(data);
        }
        _subBalance(from, value);
        emit AssetToTimeLockEvent(from, to, value, start, end);
        return string(data);
    }

    function timeLockToTimeLock(address to, uint256 value, uint64 start, uint64 end) public returns (string memory) {
        address from = _msgSender();
        (bool ok, bytes memory data) = _timeLockToTimeLock(from, to, value, start, end);
        if (!ok) {
            return string(data);
        }
        emit TimeLockToTimeLockEvent(from, to, value, start, end);
        return string(data);
    }

    function timeLockToAsset(address to, uint256 value) public returns (string memory) {
        address from = _msgSender();
        (bool ok, bytes memory data) = _timeLockToAsset(from, value);
        if (!ok) {
            return string(data);
        }
        _addBalance(to, value);
        emit TimeLockToAssetEvent(from, to, value);
        return string(data);
    }
}
