#!/usr/bin/env python

# This is basically importblock.py from the examples, just hacked into 
# use for the dark and terrible art of unit testing. 

from pyethapp.eth_protocol import TransientBlock
import rlp
import os

class Importer(object): 

    def __init__(self, eth):
        self.eth = eth

    def run(self):
        """Import all the blocks in /blocks/"""  
        blocks = sorted(os.listdir('/blocks'), key=int)
        for block in blocks:
            print '\nIMPORTING BLOCK ' + block
            data = open('/blocks/' + block, 'r').read()
            ll = rlp.decode_lazy(data)
            transient_block = TransientBlock(ll, 0)
            #transient_block.to_block(chain.db)
            print transient_block.header.hex_hash, self.eth.chain.head.hex_hash
            _block = transient_block.to_block(self.eth.chain.env, self.eth.chain.head)
            self.eth.chain.add_block(_block)

