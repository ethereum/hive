#!/bin/bash

curl -L https://foundry.paradigm.xyz | bash
source $HOME/.bashrc
foundryup
echo "" >> $HOME/.bashrc
echo 'export PATH=$HOME/.foundry/bin:$PATH' >> $HOME/.bashrc
