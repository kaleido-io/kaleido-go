pragma solidity 0.4.19;


contract Ownership {
  ///@title Claim ownership of a thing or relinquish ownership.
  mapping(string => address) private ownershipLedger;

  function claim(string id) public returns (bool success) {
    if (ownershipLedger[id] == 0) {
      ownershipLedger[id] = msg.sender;
      success = true;
    } else {
      ownershipLedger[id] = ownershipLedger[id];
      success = false;
    }
    return success;
  }

  function relinquish(string id) public {
    if (ownershipLedger[id] == msg.sender) {
      ownershipLedger[id] = 0;
    }
  }

  function getOwner(string id) view public returns (address owner) {
    owner = ownershipLedger[id];
    return owner;
  }
}
