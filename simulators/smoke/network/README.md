# Network API Smoke Test

The network API smoke test ensures that the following hive network endpoints are working as intended: 

create / remove network
- 	POST `/testsuite/{suite}/network/{network}`
-   DELETE `/testsuite/{suite}/network/{network}`

connect / disconnect container to/from network
-   POST `/testsuite/{suite}/network/{network}/node/{node}`
-   DELETE `/testsuite/{suite}/network/{network}/node/{node}`

get IP address of container on network
-   GET `/testsuite/{suite}/network/{network}/node/{node}`