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

There are two versions depending on the situation as described above. If the target enode is known, the test attempts to ping and waits for a pong *from that enode.* If the target is not known, it pings the target ip and waits for a pong response.

Fail: 
- No pong within timeout in both cases. 
- No bonding ping received within 20s.
- Pong from incorrect target enode in the first case.
- Ping from incorrect target enode in the first case.
- Packet received >1280 bytes
- Pong received with missing or incorrect ping hash
- Packet expirations in the past.


#### v4002 
This test attempts to send a ping from a hitherto unknown source node with incorrect information in the `to` field of the ping. 

Fail:
- A ping or pong received within 20s.

#### v4003
This test attempts to send a ping from a hitherto unknown source node with incorrect information in the `from` field of the ping. However, the 'from' field should be ignored by current versions of discovery v4 because its contents are not reliable. 

The test case criteria is the same as `v4001`


#### v4004
This test attempts to send a valid ping from a hitherto unknown source node with additional fields extended onto the packet. Implementations should ignore additional fields to allow for backward compatibility in future extensions.

The test case criteria is the same as `v4001`

#### v4005
This test verifies that pinging the target with additional fields works irrespective of the `from` fields contents.

The test case criteria is the same as `v4001`

#### v4006
This test verifies that pinging the target with additional fields does not affect the behaviour in the case of an incorrect `to` field.

The test case criteria is the same as `v4002`

#### v4007
This test case attempts a find neighbours request prior to endpoint verification (hitherto unknown source node)

Fail:
- A neighbours response is received within 500ms.

#### v4008
This test makes sure that the contents of unsolicited neighbours packets does not find its way into the client's table. A `neighbours` packet is sent with false neighbours information. A find neighbours request is then sent to the target.

Fail:
- No neighbours response within 500ms
- Neighbours reponse contains information from the the fake neighbours packet.

#### v4009
After bonding completes, ping the target node with incorrect `to`

Fail:
- A pong response is received.

#### v4010
After bonding completes, ping the target with a ping packet signed with a different id than what was used in the ping bond.

In a real-life situation, this would mean that the udp endpoint hosts multiple nodes, or a client is dynamically changing its id. Both cases should not be supported, so the packet should be rejected.

Fail:
- A pong response is received.

#### v4011
This test verifies that the default behaviour of ignoring `from` fields is unaffected by the bonding process. After bonding, ping the target with a different `from` endpoint. 

Fail:
- No pong response is received.

#### v4012
This test calls find neighbours on a target after the bonding process is completed. The neighbours response is expected.

Fail:
- No neighbours response is received. 
- TODO: - need to add a bootnode container with a predetermined set of neighbours. Verify that the neighbours response contains those neighbours.


#### v4013
This tests the bond 'eviction,' where after 24 hours the bonded node is no longer considered bonded. After the ping/pong exchange, attempt to dynamically change the client container time, assuming that the client container is run using libfaketime. 

Fail:
- After the bond and time change, a ping should result in both ping and pong responses (re-bond)

#### v4014
Test pinging with a past expiration. 

Fail: 
- Client responds with pong.

#### v4015
Test a find neighbours call with a past expiration.

Fail:
- Client responds with neighbours.





## RLPx



