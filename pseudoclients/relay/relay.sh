#!/bin/bash


# This script assumes the following environment variables:
#  - HIVE_RELAY_IP = the destination IP
#  - HIVE_RELAY_UDP = the destination PORT
#

# Immediately abort the script on any error encountered
set -e

echo "Starting UDP relay from 30303 to $HIVE_RELAY_IP:$HIVE_RELAY_UDP"

socat -v UDP4-RECVFROM:30303,fork UDP4-SENDTO:$HIVE_RELAY_IP:$HIVE_RELAY_UDP 

echo "Shutting down UDP relay"
