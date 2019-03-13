# Relay
The relay client uses socat to listen to traffic on the standard ports
and forward that traffic to another destination.

The HIVE_RELAY_IP and HIVE_RELAY_UDP environment variables must be 
set to specify where the traffic should be relayed to.

This initial version handles UDP traffic only, and in one direction.

