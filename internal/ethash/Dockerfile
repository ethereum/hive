# Docker container spec for building an ethash DAG for the very first epoch that
# is needed by the various simulator to prevent miners from stalling till eternity.
#
# Callers need to:
#   - Bind /root/.ethash to an external volume for cache reuse
#   - Forward UID envvar to reown newly generated ethash files
FROM ethereum/client-go

# Define the tiny startup script to generate the DAG and reown it
RUN \
  echo '#!/bin/sh'                          > /ethash.sh && \
	echo 'set -e'                            >> /ethash.sh && \
	echo 'geth makedag 1 /root/.ethash'      >> /ethash.sh && \
  echo 'if [ "$UID" != "0" ]; then'        >> /ethash.sh && \
	echo '  adduser -u $UID -D ethash'       >> /ethash.sh && \
	echo '  chown -R ethash /root/.ethash/*' >> /ethash.sh && \
  echo 'fi'                                >> /ethash.sh && \
	chmod +x /ethash.sh

ENTRYPOINT ["/ethash.sh"]
