#!/bin/bash

# Immediately abort the script on any error encountered
set -e

/usr/bin/nimbus_validator_client --version | grep -Po ".*\bv?\d+\.\S+" | sed "s/ /_/g" | tr '\n' '/'