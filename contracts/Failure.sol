// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

contract Failure {
    function fail() public payable {
        revert("big fail!");
    }

    function burn(uint256 _amount) public payable {
        uint256 i = 0;
        uint256 initialGas = gasleft();
        while (initialGas - gasleft() < _amount) {
            ++i;
        }
    }
}
