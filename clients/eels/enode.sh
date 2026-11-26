#!/bin/bash

# EELS runs the execution specification behind the Engine API only; there
# is no devp2p stack, so no enode URL can be reported.
echo "eels does not support peer-to-peer networking" >&2
exit 1
