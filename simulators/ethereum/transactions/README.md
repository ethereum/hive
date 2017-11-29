# Cross client testing

Testing implementations of Ethereum clients has been a difficult task. A standard testcase repository has been maintained,
but the actual test-execution has been left unspecified.

Each client has had to implement client-specific test-harnesses to perform the tests:

* The actual execution of the testcase, which includes creating necessary `prestate`.
* The verificaton of the testcase, which includes verifying various things, such as:
  * `poststate` state root
  * `lastblock` hash
  * Various `account`s, and `storage`.

## A generalised test

The [Ethereum Standard Testcases](https://github.com/ethereum/tests)testcases have now been transformed so that the bulk of all tests can be expressed as blocktests. The framework consists
of the following parts.

1. The testcase sources. These are basically the current testcases (testfillers). They contain the semantics of each test,
  and could e.g contain contract source code or individual transactions to be applied to a state. These tests should contain
  sufficient information for a human to be able to interpret the meaning of the test, and how to go about debugging a failed test.

2. The testcase artefacts. These are a transformation of the testcases (testfillers), onto the following form:

      `prestate` + `block(s)` => `postState`.

3. The execution of cross-client testing is then performed using Hive. The hive framework simply performs the following.

* For each client, for each testcase:
   * Create genesis by combining 'pre' and 'genesisBlockHeader'.
   * Create ruleset, according to testcase (Frontier, Homestead, Tangerine or Spurious), and set Hive `ENV` variables for the node
   * Instantiate hive-node (client), and write generated files to the node filesystem
   * The node will import blocks upon startup
   * Verify preconditions at block(0), using standard web3 api
   * Verify postconditions at block(n), using standard web3 api
   * Emit testcase subresult to Hive

The python-framework in `ethereum/consensus` uses `7` paralell threads to execute testcases.

The Hive framework then outputs the results-report as a json-file, which can be packaged with a HTML viewer and client-logs for analysis of all test failures.  

An example of such a framework is this: https://github.com/holiman/testreport_template/ .

The benefits of this approach is that there is no need for client-implementations to create a bespoke testing framework, as long as they
conform to the requirements of being a hive-node. Which are, basically:

- Ability to import blocks.
- Ability to configure ruleset via commandline/config.
- Standard web3 methods (getBlock).

Onboarding a client into Hive is pretty simple:

1. Create a Docker file (as minimal as possible) which installs the client
2. Create a shell script which can boot the client according to given `ENV` variables.
  * The `ENV` variables contain information about which ruleset to use (Frontier, Homestead, Tangerine etc). The script is also responsible for importing genesis and blocks from the filesystem.

## Todo / Future work

* Paremeterize execution
  * Set number of threads
  * Ability to execute a chosen subset of tests
* Measure and report testcase timings, to have metrics about which tests to skip to decrease execution time
* Onboard more clients
  * EthereumJ (see https://github.com/ethereum/ethereumj/issues/728)
  * Python
