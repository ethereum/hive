# Docker container spec for building the local branch of EthereumJ.
#
# As current Docker limitations don't allow us to use local resources otside context during the build,
# you should prepare custom Harmony built with your local EthereumJ using ./build.sh from this directory
# (JDK should be already installed).
#
#  Finally, you can use harmony.ether.camp.tar for Hive tests in the following manner:
#  hive --docker-noshell=true --client=ethereumj:local --override=$HOME/harmony.ether.camp.tar  --smoke
#
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.
FROM anapsix/alpine-java:8_jdk_unlimited

# Setup everything we need for running Harmony
RUN apk add --update bash jq ca-certificates linux-headers

# Inject the startup script and everything else
ADD harmony.sh /harmony.sh
ADD version.json /version.json
ADD logback.xml /logback.xml
RUN chmod +x /harmony.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303

ENTRYPOINT ["/harmony.sh"]

#EOF
