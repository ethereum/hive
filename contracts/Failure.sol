// SPDX-License-Identifier: MIT
pragma solidity ^0.8.9;

contract Failure {
    function fail() public payable {
        revert("big fail!");
    }
}
