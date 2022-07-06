This directory contains the hive proxy. The proxy
enables communication between the hive command-line tool
and the docker network.

The proxy code is contained in a separate Go module and
must reside in a subdirectory in order to be accepted by
go:embed.

