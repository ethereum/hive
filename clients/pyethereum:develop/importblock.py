#!/usr/bin/env python

# This is basically importblock.py from the examples, just hacked into
# use for the dark and terrible art of unit testing.

from ethereum.block import BlockHeader
from ethereum.transactions import Transaction
from pyethapp.eth_protocol import TransientBlock
import rlp
import os

class Importer(object):

    def __init__(self, eth):
        self.eth = eth

    def run(self):
        """Import all the blocks in /blocks/"""
        print('\nImport all the blocks in /blocks/')
        blocks = sorted(os.listdir('/blocks'), key=str)
        for block in blocks:
            data = open('/blocks/' + block, 'r').read()
            block_data = rlp.decode_lazy(data)
            header = BlockHeader.deserialize(block_data[0])
            transactions = rlp.sedes.CountableList(Transaction).deserialize(block_data[1])
            uncles = rlp.sedes.CountableList(BlockHeader).deserialize(block_data[2])
            transient_block = TransientBlock(header, transactions, uncles, 0)
            print '\nIMPORTING BLOCK %s (%s)' % (block, transient_block.header.hex_hash)
            self.eth.chain.add_block(transient_block.to_block())
