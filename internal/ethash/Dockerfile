# Docker container spec for building an ethash DAG for the very first epoch that
# is needed by the various simulator to prevent miners from stalling till eternity.
#
# Callers need to:
#   - Bind /root/.ethash to an external volume for cache reuse
#   - Forward UID envvar to reown newly generated ethash files
FROM ethereum/client-go

# Define the tiny startup script to generate the DAG and reown it
RUN \
  echo '#!/bin/sh'                          > /root/ethash.sh && \
  echo 'set -e'                            >> /root/ethash.sh && \
  echo 'geth makedag 1 /root/.ethash'      >> /root/ethash.sh && \
  echo 'if [ "$UID" != "0" ]; then'        >> /root/ethash.sh && \
  echo '  adduser -u $UID -D ethash'       >> /root/ethash.sh && \
  echo '  chown -R ethash /root/.ethash/*' >> /root/ethash.sh && \
  echo 'fi'                                >> /root/ethash.sh && \
  chmod +x /root/ethash.sh

ENTRYPOINT ["/root/ethash.sh"]
