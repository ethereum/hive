# This benchmark imports a pre-generated chain containing the SnailTracer path
# trace contract.
#
# For details see https://github.com/karalabe/snailtracer
#
# To build a new version of the SnailTracer benchmark:
#  - Create a new private chain with the genesis.json from here
#  - Import the account from the keys.tar.xz archive
#  - Deploy a new contract in a geth console:
#    - var st  = web3.eth.contract([...])
#    - var sti = st.new(1024, 768, {"data": "0x...", "from": eth.accounts[0], "gas": 10000000})
#    - miner.start(1) ...wait... miner.stop()
#  - Export the block with the deployed contract
FROM mhart/alpine-node

# Install web3 to allow interacting with the tracer
RUN apk add --no-cache git
RUN npm install web3

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json
ADD chain.rlp /chain.rlp

# Add the benchmark controller
ADD benchmark.js /benchmark.js
ENTRYPOINT ["node", "/benchmark.js"]
