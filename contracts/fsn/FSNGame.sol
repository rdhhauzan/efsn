pragma solidity ^0.5.4;

import "https://github.com/cross-chain/efsn/contracts/fsn/FSNContract.sol";

contract CommonBase {
    enum gameOutcome {
        win,
        draw,
        lose
    }
    enum gameFunc {
        openGame,
        closeWin,
        closeDraw,
        closeLose,
        deleteGame,
        shareBonus,
        adjustBatchCount
    }
    address payable owner;
    constructor() public {
        owner = msg.sender;
    }
    modifier onlyOwner {
        require(msg.sender == owner, "only owner");
        _;
    }
}

contract Game is CommonBase, FSNContract {
    event LogBet(address indexed _player, gameOutcome _outcome, uint256 _value);
    event LogShareBonus(address indexed _player, uint256 _value);
    event LogMarkerBonus(uint256 _value);
    event LogShareBonusState(uint256 _totalStakers, uint256 _finishStakers);

    struct itmap {
        mapping(address => uint256) data;
        address[] keys;
        uint256 totalValue;
    }
    mapping(uint8 => itmap) stakeMap;

    address payable bookmarker;
    address gameAddress;
    string gameID;
    string gameInfo;
    bool isOpen = false;
    bool isClose = false;
    bool isFinish = false;
    gameOutcome outcome;

    struct FinalState {
        uint256 totalStake;
        uint256 sharedBonus;
        uint256 markerBonus;
        uint256 totalStakers;
        uint256 finishStakers;
    }
    FinalState public finalState;
    uint256 finishSharedBonus;
    uint256 public batchCount = 50;
    bytes32 public paymentAsset;
    uint64 public endOfTimeLock;

    constructor(string memory _gameID, string memory _gameInfo, bytes32 _assetID, uint64 _endOfTimeLock) mustCallIndirectly public {
        bookmarker = tx.origin;
        gameAddress = address(this);
        gameID = _gameID;
        gameInfo = _gameInfo;
        paymentAsset = _assetID;
        endOfTimeLock = _endOfTimeLock;
    }

    modifier mustNotOpen{
        require(!isOpen, "must not open");
        _;
    }

    modifier mustOpenAndNotClose{
        require(isOpen && !isClose, "must open and not close");
        _;
    }

    modifier mustCloseAndNotFinish{
        require(isClose && !isFinish, "must close and not finish");
        _;
    }

    modifier mustCallIndirectly {
        require(tx.origin != msg.sender, "must call indirectly");
        _;
    }

    function open() mustNotOpen private returns (bool success) {
        isOpen = true;
        return true;
    }

    function close(gameOutcome _outcome) mustOpenAndNotClose private returns (bool success) {
        isClose = true;
        outcome = _outcome;
        calcFinalState();
        shareBonus();
        return true;
    }

    function deleteGame() private view returns (bool success) {
        require(!isOpen || isFinish, "game not deletable");
        return true;
    }

    function receiveAsset(bytes32 assetID, uint64 startTime, uint64 endTime, SendAssetFlag flag, uint256[] memory extraInfo) payable public returns (bool success) {
        (flag); // silence warning of Unused function parameter
        require(msg.value >= 1 ether, "value too small");
        require(assetID == paymentAsset, "wrong asset");
        require(endTime == endOfTimeLock && startTime <= block.timestamp && startTime <= endTime, "wrong time range");
        require(extraInfo.length == 1 && extraInfo[0] <= 2, "invalid outcome");
        gameOutcome _outcome = gameOutcome(extraInfo[0]);
        itmap storage stake = getStorage(_outcome);
        address key = msg.sender;
        bool exist = stake.data[key] > 0;
        stake.data[key] += msg.value;
        stake.totalValue += msg.value;
        if (!exist) {
            stake.keys.push(key);
        }
        emit LogBet(key, _outcome, msg.value);
        return true;
    }

    function adjustBatchCount(uint256 _batchCount) private returns (bool success) {
        require(_batchCount != 0, "batchCount is 0");
        batchCount = _batchCount;
        return true;
    }

    function getInfo() public view returns (address _gameAddress, string memory _gameID, string memory _gameInfo) {
        return (gameAddress, gameID, gameInfo);
    }

    function getState() public view returns (bool _isOpen, bool _isClose, bool _isFinish, gameOutcome _outcome) {
        return (isOpen, isClose, isFinish, outcome);
    }

    function getStorage(gameOutcome _outcome) private view returns (itmap storage) {
        return stakeMap[uint8(_outcome)];
    }

    function getAllStake(gameOutcome _outcome) public view returns (uint256 totalStake, uint256 numStakers) {
        itmap storage stake = getStorage(_outcome);
        return (stake.totalValue, stake.keys.length);
    }

    function getAllBonus() public view returns (uint256 totalBonus) {
        return getStorage(gameOutcome.win).totalValue + getStorage(gameOutcome.draw).totalValue + getStorage(gameOutcome.lose).totalValue;
    }

    function calcFinalState() private {
        (finalState.totalStake, finalState.totalStakers) = getAllStake(outcome);
        uint256 totalBonus = getAllBonus();
        if (finalState.totalStake == 0) {
            finalState.markerBonus = totalBonus;
            finalState.sharedBonus = 0;
        } else {
            finalState.markerBonus = totalBonus / 10;
            finalState.sharedBonus = totalBonus - finalState.markerBonus;
        }
    }

    function shareBonus() mustCloseAndNotFinish private returns (bool success) {
        uint256 endPos = finalState.finishStakers + batchCount;
        if (endPos > finalState.totalStakers) {
            endPos = finalState.totalStakers;
        }
        bool lastTurn = (endPos == finalState.totalStakers);
        if (finalState.totalStake != 0) {
            address playerAddr;
            uint256 playerStake;
            uint256 value;
            itmap storage stake = getStorage(outcome);
            for (uint256 i = finalState.finishStakers; i < endPos; i++) {
                playerAddr = stake.keys[i];
                playerStake = stake.data[playerAddr];
                value = finalState.sharedBonus * playerStake / finalState.totalStake;
                (success,) = _sendAsset(paymentAsset, playerAddr, value, 0, endOfTimeLock, SendAssetFlag.UseTimeLock);
                if (success) {
                    finishSharedBonus += value;
                    emit LogShareBonus(playerAddr, value);
                }
            }
            finalState.finishStakers = endPos;
        }
        if (lastTurn) {
            uint256 value = finalState.markerBonus + (finalState.sharedBonus - finishSharedBonus);
            (success,) = _sendAsset(paymentAsset, bookmarker, value, 0, endOfTimeLock, SendAssetFlag.UseTimeLock);
            if (success) {
                emit LogMarkerBonus(value);
            }
            isFinish = true;
        } else {
            emit LogShareBonusState(finalState.totalStakers, finalState.finishStakers);
        }
        return true;
    }

    function callByOwner(gameFunc _func, uint256 _batchCount) onlyOwner mustCallIndirectly external returns (bool success) {
        require(tx.origin == bookmarker, "must origin from bookmarker");
        if (_func ==gameFunc.openGame) {
            return open();
        }
        if (_func ==gameFunc.closeWin) {
            return close(gameOutcome.win);
        }
        if (_func ==gameFunc.closeDraw) {
            return close(gameOutcome.draw);
        }
        if (_func ==gameFunc.closeLose) {
            return close(gameOutcome.lose);
        }
        if (_func ==gameFunc.deleteGame) {
            return deleteGame();
        }
        if (_func ==gameFunc.shareBonus) {
            return shareBonus();
        }
        if (_func ==gameFunc.adjustBatchCount) {
            return adjustBatchCount(_batchCount);
        }
        require(false, "unknown function");
    }
}

contract Bookmaker is CommonBase {
    event LogOpenGame(string _gameID, address _gameAddress);
    event LogCloseGame(string _gameID, address _gameAddress, gameOutcome _outcome);
    event LogDeletelGame(string _gameID, address _gameAddress);
    event LogSetBatchCount(string _gameID, address _gameAddress, uint256 _batchCount);

    mapping(string => address) gameMap;

    function openGame(string memory _id, string memory _info, bytes32 _asset, uint64 _datesOfTimeLock) onlyOwner public returns (bool success) {
        require(gameMap[_id] == address(0), "game exist");
        require(bytes(_id).length <= 256, "game id too long");
        require(bytes(_info).length <= 1024, "game info too long");
        require(_datesOfTimeLock >= 30 && _datesOfTimeLock <=365, "invalid dates of timelock long");
        Game game = new Game(_id, _info, _asset, uint64(block.timestamp)+_datesOfTimeLock*24*3600);
        game.callByOwner(gameFunc.openGame, 0);
        gameMap[_id] = address(game);
        emit LogOpenGame(_id, address(game));
        return true;
    }

    function closeWin(string memory _id) onlyOwner public returns (bool success) {
        getGame(_id).callByOwner(gameFunc.closeWin, 0);
        emit LogCloseGame(_id, gameMap[_id], gameOutcome.win);
        return true;
    }

    function closeDraw(string memory _id) onlyOwner public returns (bool success) {
        getGame(_id).callByOwner(gameFunc.closeDraw, 0);
        emit LogCloseGame(_id, gameMap[_id], gameOutcome.draw);
        return true;
    }

    function closeLose(string memory _id) onlyOwner public returns (bool success) {
        getGame(_id).callByOwner(gameFunc.closeLose, 0);
        emit LogCloseGame(_id, gameMap[_id], gameOutcome.lose);
        return true;
    }

    function deleteGame(string memory _id) onlyOwner public returns (bool success) {
        getGame(_id).callByOwner(gameFunc.deleteGame, 0);
        emit LogDeletelGame(_id, gameMap[_id]);
        delete(gameMap[_id]);
        return true;
    }

    function shareBonus(string memory _id) onlyOwner public returns (bool success) {
        return getGame(_id).callByOwner(gameFunc.shareBonus, 0);
    }

    function setBatchCount(string memory _id, uint256 _count) onlyOwner public returns (bool success) {
        getGame(_id).callByOwner(gameFunc.adjustBatchCount, _count);
        emit LogSetBatchCount(_id, gameMap[_id], _count);
        return true;
    }

    function getGame(string memory _id) internal view returns (Game) {
        require(gameMap[_id] != address(0), "game not exist");
        return Game(gameMap[_id]);
    }

    function getGameAddress(string memory _id) public view returns (address gameAddress) {
        return gameMap[_id];
    }
}
