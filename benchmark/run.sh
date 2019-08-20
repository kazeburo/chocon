#!/bin/bash

set -eu

# This corresponds to the latency of a single-trip communication.
# If this is set to 10ms, the round-trip latency will be 20ms.
LATENCY=10ms

docker-compose up -d --build

# Make sure that the containers get shut down on error.
clean_up () {
    docker-compose down
    exit 1
} 
trap clean_up EXIT

# Simulate high-latency network.
# Only having `docker exec server ...` is not enough as 'tc' command only affects the outbound communication.
docker exec server tc qdisc add dev eth0 root netem delay $LATENCY
docker exec chocon tc qdisc add dev eth0 root netem delay $LATENCY
docker exec client tc qdisc add dev eth0 root netem delay $LATENCY

# Wait until the servers are ready.
sleep 3

echo "client -> chocon -> [$LATENCY latency] -> server"
docker exec chocon ab -n 100 -H "Host: server.ccnproxy.local" http://localhost:3000/

echo "client -> [$LATENCY latency] -> server"
docker exec client ab -n 100 http://server/

docker-compose down
