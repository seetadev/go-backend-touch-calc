#!/bin/bash

# Redis setup script for LibP2P integration

echo "Setting up Redis for LibP2P integration..."

# Connect to Redis and setup keys
docker-compose exec redis redis-cli << EOF
# Create namespace for LibP2P nodes
SET libp2p:config:load_balancer "nginx"
SET libp2p:config:session_persistence "ip_hash"
SET libp2p:config:node_count "3"

# Initialize node discovery
SADD libp2p:discovery:active_nodes "node1" "node2" "node3"

# Set TTL for node heartbeats
EXPIRE libp2p:nodes:1 3600
EXPIRE libp2p:nodes:2 3600
EXPIRE libp2p:nodes:3 3600

# Create connection pool settings
HSET libp2p:config:pool "max_connections" 100
HSET libp2p:config:pool "idle_timeout" 300
HSET libp2p:config:pool "keepalive" 1

EOF

echo "Redis setup completed!"
