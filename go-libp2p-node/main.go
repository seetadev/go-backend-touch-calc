package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/libp2p/go-libp2p/p2p/protocol/ping"
    "github.com/libp2p/go-libp2p/p2p/transport/websocket"
    "github.com/multiformats/go-multiaddr"
)

type LibP2PNode struct {
    host.Host
    ctx         context.Context
    cancel      context.CancelFunc
    redisClient *redis.Client
    nodeID      string
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    nodeID := os.Getenv("NODE_ID")
    if nodeID == "" {
        nodeID = "1"
    }

    // Redis connection
    redisURL := os.Getenv("REDIS_URL")
    if redisURL == "" {
        redisURL = "redis://localhost:6379"
    }

    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        log.Fatalf("Failed to parse Redis URL: %v", err)
    }

    rdb := redis.NewClient(opt)
    
    // Test Redis connection
    _, err = rdb.Ping(ctx).Result()
    if err != nil {
        log.Fatalf("Failed to connect to Redis: %v", err)
    }

    node := &LibP2PNode{
        ctx:         ctx,
        cancel:      cancel,
        redisClient: rdb,
        nodeID:      nodeID,
    }

    if err := node.start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }
}

func (n *LibP2PNode) start() error {
    // Parse listen addresses from environment
    listenAddrs := []string{"/ip4/0.0.0.0/tcp/0"}
    if envAddrs := os.Getenv("LISTEN_ADDRESSES"); envAddrs != "" {
        listenAddrs = strings.Split(envAddrs, ",")
    }

    // Create libp2p host with WebSocket support
    opts := []libp2p.Option{
        libp2p.ListenAddrStrings(listenAddrs...),
        libp2p.Ping(false),
        libp2p.Transport(websocket.New),
    }

    h, err := libp2p.New(opts...)
    if err != nil {
        return fmt.Errorf("failed to create libp2p node: %v", err)
    }
    n.Host = h
    defer h.Close()

    // Setup ping service
    pingService := &ping.PingService{Host: h}
    h.SetStreamHandler(ping.ID, pingService.PingHandler)

    // Register this node in Redis
    if err := n.registerNode(); err != nil {
        log.Printf("Failed to register node in Redis: %v", err)
    }

    // Start periodic heartbeat
    go n.heartbeat()

    fmt.Printf("LibP2P Node %s started\n", n.nodeID)
    fmt.Printf("Peer ID: %s\n", h.ID())
    for _, addr := range h.Addrs() {
        fmt.Printf("Listening on: %s/p2p/%s\n", addr, h.ID())
    }

    // Handle connections and discovery
    go n.handleDiscovery()

    // Wait for shutdown signal
    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
    <-c

    fmt.Printf("\nShutting down LibP2P node %s...\n", n.nodeID)
    
    // Unregister from Redis
    n.unregisterNode()
    
    return nil
}

func (n *LibP2PNode) registerNode() error {
    nodeInfo := map[string]interface{}{
        "node_id":    n.nodeID,
        "peer_id":    n.Host.ID().String(),
        "addresses":  n.getAddressStrings(),
        "timestamp":  time.Now().Unix(),
        "status":     "active",
    }

    key := fmt.Sprintf("libp2p:nodes:%s", n.nodeID)
    return n.redisClient.HMSet(n.ctx, key, nodeInfo).Err()
}

func (n *LibP2PNode) unregisterNode() error {
    key := fmt.Sprintf("libp2p:nodes:%s", n.nodeID)
    return n.redisClient.Del(n.ctx, key).Err()
}

func (n *LibP2PNode) heartbeat() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := n.updateHeartbeat(); err != nil {
                log.Printf("Failed to update heartbeat: %v", err)
            }
        case <-n.ctx.Done():
            return
        }
    }
}

func (n *LibP2PNode) updateHeartbeat() error {
    key := fmt.Sprintf("libp2p:nodes:%s", n.nodeID)
    return n.redisClient.HSet(n.ctx, key, "timestamp", time.Now().Unix()).Err()
}

func (n *LibP2PNode) handleDiscovery() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            n.discoverPeers()
        case <-n.ctx.Done():
            return
        }
    }
}

func (n *LibP2PNode) discoverPeers() {
    keys, err := n.redisClient.Keys(n.ctx, "libp2p:nodes:*").Result()
    if err != nil {
        log.Printf("Failed to discover peers: %v", err)
        return
    }

    for _, key := range keys {
        if strings.Contains(key, n.nodeID) {
            continue // Skip self
        }

        nodeInfo, err := n.redisClient.HGetAll(n.ctx, key).Result()
        if err != nil {
            continue
        }

        peerID := nodeInfo["peer_id"]
        if peerID == "" {
            continue
        }

        // Try to connect to discovered peer
        go n.connectToPeer(peerID, nodeInfo["addresses"])
    }
}

func (n *LibP2PNode) connectToPeer(peerIDStr, addressesStr string) {
    peerID, err := peer.Decode(peerIDStr)
    if err != nil {
        return
    }

    // Check if already connected
    if n.Host.Network().Connectedness(peerID) == 1 { // Connected
        return
    }

    addresses := strings.Split(addressesStr, ",")
    for _, addrStr := range addresses {
        addr, err := multiaddr.NewMultiaddr(addrStr)
        if err != nil {
            continue
        }

        peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
        if err != nil {
            continue
        }

        ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
        err = n.Host.Connect(ctx, *peerInfo)
        cancel()

        if err == nil {
            log.Printf("Connected to peer %s", peerID)
            break
        }
    }
}

func (n *LibP2PNode) getAddressStrings() string {
    var addrs []string
    for _, addr := range n.Host.Addrs() {
        addrs = append(addrs, addr.String())
    }
    return strings.Join(addrs, ",")
}
