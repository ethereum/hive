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
        print('\nImport all the blocks in /blocks/')
        blocks = sorted(os.listdir('/blocks'), key=str)
        for block in blocks:
            data = open('/blocks/' + block, 'r').read()
            ll = rlp.decode_lazy(data)
            transient_block = TransientBlock(ll, 0)
            parent = self.eth.chain.get(transient_block.header.prevhash)
            print '\nIMPORTING BLOCK %s (%s)' % (block, transient_block.hex_hash)
            print "New block hash: " + transient_block.header.hex_hash()
            print "Parent hash: " + parent.header.hex_hash()
            _block = transient_block.to_block(self.eth.chain.env, parent)
            self.eth.chain.add_block(_block)
