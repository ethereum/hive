# DevP2P test set file
This file describes the set of tests carried out by the devp2p hive validator.
It is organised by test group name, then by a test code or number.
These groups and codes are used in the golang test file to identity the test carried out.
Each test here describes the purpose and criteria for failure or success.

The group and test identifiers can be used to specify restrictions on which tests to run. For example, to run only Discovery tests, in tests.sh or from a standalone launch, run 

`devp2p.test -test.v -test.run Discovery -enodeTarget "$TARGET_ENODE" -targetIP "$HIVE_CLIENT_IP"`



## Discovery 
The `Discovery` subtest covers discovery v4 and v5 tests.

### discoveryv4

The `discoveryv4` subtest covers tests described in this section. 

Some tests attempt to show how the target node responds to packets when the target enode identifier is unknown. Hive client images should have an enode.sh file in their root, which should include a client specific method of getting the enode id. For example, Parity and geth images include enode.sh files that utilise curl to ask the `HIVE_CLIENT_IP` for a client-specific admin api response. However, this might not always be possible. In the case that the target enode is unknown, the test suite attempts an initial ping to an unspecified enode and then recovers the enode id (pubkey) from the pong response. In this case the recovered id is used as the basis for subsequent tests, but it does leave open the possibility that clients may be able to 'fake' their identities on the basis of new incoming connections.

#### v4001
This test attempts to ping the target from a hitherto unknown source node with `from` and `to` set correctly.

There are two versions depending on the situation as described above. If the target enode is known, the test attempts to ping and waits for a pong *from that enode.* If the target is not known, it pings the target ip and waits for a pong response, and if one is obtained within the timeout it recovers the enode id for successive tests.

Fail: 
- No pong within timeout in both cases. 
- No bonding ping received within 20s.
- Pong from incorrect target enode in the first case.
- Ping from incorrect target enode in the first case.
- Ping `from` field with incorrect id.
- Packet received >1280 bytes
- Pong received with missing or incorrect ping hash
- Packet expirations in the past.


#### v0002 






## RLPx



