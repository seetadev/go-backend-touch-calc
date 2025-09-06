package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/libp2p/go-libp2p/p2p/protocol/ping"
    "github.com/multiformats/go-multiaddr"
)

func main() {
    ctx := context.Background()

    node, err := libp2p.New(
        libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
        libp2p.Ping(false),
    )
    if err != nil {
        log.Fatalf("Failed to create libp2p node: %v", err)
    }
    defer node.Close()

    pingService := &ping.PingService{Host: node}
    node.SetStreamHandler(ping.ID, pingService.PingHandler)

    fmt.Printf("Go-libp2p node started\n")
    fmt.Printf("Peer ID: %s\n", node.ID())
    for _, addr := range node.Addrs() {
        fmt.Printf("Listening on: %s/p2p/%s\n", addr, node.ID())
    }

    if len(os.Args) > 1 {
        remotePeer := os.Args[1]
        if err := connectAndPing(ctx, node, pingService, remotePeer); err != nil {
            log.Fatalf("Failed to ping remote peer: %v", err)
        }
    }

    c := make(chan os.Signal, 1)
    signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
    <-c
    fmt.Println("\nShutting down go-libp2p node...")
}

func connectAndPing(ctx context.Context, node host.Host, pingService *ping.PingService, remotePeer string) error {
    addr, err := multiaddr.NewMultiaddr(remotePeer)
    if err != nil {
        return fmt.Errorf("invalid multiaddr: %v", err)
    }

    peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
    if err != nil {
        return fmt.Errorf("failed to get peer info: %v", err)
    }

    fmt.Printf("Connecting to: %s\n", remotePeer)
    if err := node.Connect(ctx, *peerInfo); err != nil {
        return fmt.Errorf("failed to connect: %v", err)
    }

    fmt.Printf("Sending 5 ping messages to %s\n", peerInfo.ID)
    ch := pingService.Ping(ctx, peerInfo.ID)
    
    for i := 0; i < 5; i++ {
        select {
        case res := <-ch:
            if res.Error != nil {
                fmt.Printf("Ping %d failed: %v\n", i+1, res.Error)
            } else {
                fmt.Printf("Ping %d successful - RTT: %v\n", i+1, res.RTT)
            }
        case <-time.After(10 * time.Second):
            fmt.Printf("Ping %d timed out\n", i+1)
        }
    }
    
    return nil
}
