#!/bin/bash

set -eu

# This corresponds to the latency of a single-trip communication.
# If this is set to 10ms, the round-trip latency will be 20ms.
LATENCY=10ms

docker-compose up -d
# Make sure that the containers get shut down on error.
clean_up () {
    docker-compose down
} 
trap clean_up EXIT

# Simulate high-latency network.
# Only having `docker exec server ...` is not enough as 'tc' command only affects the outbound communication.
docker exec server tc qdisc add dev eth0 root netem delay $LATENCY
docker exec chocon tc qdisc add dev eth0 root netem delay $LATENCY
docker exec client tc qdisc add dev eth0 root netem delay $LATENCY

# Wait until the servers are ready.
sleep 3

# Regex pattern to filter the output of 'ab' commands.
AB_FILTER_PATTERN="^Time per request.*\(mean\)$"

echo "client -> [$LATENCY latency] -> server (http)"
echo "first request"
docker exec client ab -n 1 http://server/ | grep -E "$AB_FILTER_PATTERN"
echo "following 100 requests"
docker exec client ab -n 100 http://server/ | grep -E "$AB_FILTER_PATTERN"
echo ""

echo "client -> chocon -> [$LATENCY latency] -> server (http)"
echo "first request"
docker exec chocon ab -n 1 -H "Host: server.ccnproxy.local" http://localhost:3000/ | grep -E "$AB_FILTER_PATTERN"
echo "following 100 requests"
docker exec chocon ab -n 100 -H "Host: server.ccnproxy.local" http://localhost:3000/ | grep -E "$AB_FILTER_PATTERN"
echo ""

echo "client -> [$LATENCY latency] -> server (https)"
echo "first request"
docker exec client ab -n 1 https://server/ | grep -E "$AB_FILTER_PATTERN"
echo "following 100 requests"
docker exec client ab -n 100 https://server/ | grep -E "$AB_FILTER_PATTERN"
echo ""

echo "client -> chocon -> [$LATENCY latency] -> server (https)"
echo "first request"
docker exec chocon ab -n 1 -H "Host: server.ccnproxy-secure.local" http://localhost:3000/ | grep -E "$AB_FILTER_PATTERN"
echo "following 100 requests"
docker exec chocon ab -n 100 -H "Host: server.ccnproxy-secure.local" http://localhost:3000/ | grep -E "$AB_FILTER_PATTERN"
