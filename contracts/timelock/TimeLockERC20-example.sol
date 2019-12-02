pragma solidity ^0.5.0;

import "https://github.com/cross-chain/openzeppelin-contracts/contracts/token/ERC20/ERC20Detailed.sol";
import "https://github.com/cross-chain/efsn/contracts/timelock/TimeLockERC20.sol";

contract ZYToken is TimeLockERC20, ERC20Detailed {
    constructor() public TimeLockERC20() ERC20Detailed("ZY Token", "ZYT", 18) {
        _mint(msg.sender, 50000000*10**18);
    }
}
