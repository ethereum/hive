"""The spec reference's PBT root for a genesis allocation.

Builds the state through execution-specs' own state_pbt model, the path its
binary_trie_vectors.json roots come from, so nothing here re-derives an
embedding rule:

    PYTHONPATH=$SPECS/src $SPECS/.venv/bin/python pbt_root.py genesis.json
"""

import json
import sys

from ethereum_types.bytes import Bytes, Bytes32
from ethereum_types.numeric import U256, Uint

from ethereum.state import Account, Address
from ethereum.state_pbt import State, set_account, set_storage, state_root, store_code


def number(v):
    return int(v, 0) if isinstance(v, str) else v


state = State()
for addr, acct in json.load(open(sys.argv[1]))["alloc"].items():
    address = Address(bytes.fromhex(addr.removeprefix("0x")))
    code_hash = store_code(state, Bytes(bytes.fromhex(acct.get("code", "0x")[2:])))
    set_account(state, address, Account(
        nonce=Uint(number(acct.get("nonce", 0))),
        balance=U256(number(acct.get("balance", 0))),
        code_hash=code_hash,
    ))
    for slot, value in acct.get("storage", {}).items():
        set_storage(state, address, Bytes32(int(slot, 16).to_bytes(32)), U256(int(value, 16)))
print("0x" + state_root(state).hex())
