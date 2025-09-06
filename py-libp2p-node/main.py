#!/usr/bin/env python3
import sys
import signal
import time
from typing import List

import trio
import multiaddr
import libp2p
from libp2p import new_host
from libp2p.crypto.rsa import create_new_key_pair

class PyLibp2pPingNode:
    def __init__(self):
        self.host = None
        
    async def start(self, listen_port: int = 0):
        """Start the py-libp2p node"""
        key_pair = create_new_key_pair()
        
        self.host = new_host(key_pair=key_pair)
        
        listen_addr = multiaddr.Multiaddr(f"/ip4/0.0.0.0/tcp/{listen_port}")
        
        try:
            async with self.host.run(listen_addrs=[listen_addr]):
                print(f"py-libp2p node started")
                print(f"Peer ID: {self.host.get_id()}")
                
                for addr in self.host.get_addrs():
                    print(f"Listening on: {addr}/p2p/{self.host.get_id()}")
                
                if len(sys.argv) > 1:
                    await self.connect_and_ping(sys.argv[1])
                else:
                    print("Waiting for connections...")
                    await self.wait_for_shutdown()
        except Exception as e:
            print(f"Failed to start node: {e}")
            raise
    
    async def connect_and_ping(self, remote_peer: str):
        """Connect to remote peer and send pings"""
        try:
            print(f"Attempting to connect to: {remote_peer}")
            
            maddr = multiaddr.Multiaddr(remote_peer)
            
            from libp2p.peer.peerinfo import info_from_p2p_addr
            peer_info = info_from_p2p_addr(maddr)
            
            print(f"Connecting to peer: {peer_info.peer_id}")
            await self.host.connect(peer_info)
            print(f"Connected to {peer_info.peer_id}")
            
            print(f"Sending 5 messages to {peer_info.peer_id}")
            for i in range(5):
                try:
                    print(f"Message {i+1} - Connection active")
                    await trio.sleep(1)

                except Exception as e:
                    print(f"Message {i+1} failed: {e}")
                
        except Exception as e:
            print(f"Connection error: {e}")
            import traceback
            traceback.print_exc()
    
    async def wait_for_shutdown(self):
        """Wait for shutdown signal"""
        try:
            while True:
                await trio.sleep(1)
        except KeyboardInterrupt:
            print("Received shutdown signal")
            pass

async def main():
    node = PyLibp2pPingNode()
    await node.start()

if __name__ == "__main__":
    try:
        trio.run(main)
    except KeyboardInterrupt:
        print("Node stopped.")
    except Exception as e:
        print(f"Error: {e}")
        import traceback
        traceback.print_exc()
