#!/bin/bash

echo "=== libp2p Interoperability Testing ==="
echo "Testing ping protocol between go-libp2p and py-libp2p"

mkdir -p ../results

echo "Test 1: go-libp2p server -> py-libp2p client"
cd ../go-libp2p-node

if [ ! -f "./go-ping-node" ]; then
    echo "Building go-libp2p node..."
    go build -o go-ping-node .
fi

./go-ping-node > ../results/go-server.log 2>&1 &
GO_PID=$!
echo "Started go node with PID: $GO_PID"
sleep 5

GO_ADDR=$(grep "Listening on: /ip4/127.0.0.1" ../results/go-server.log | head -n1 | awk '{print $3}')
echo "Go node address: $GO_ADDR"

if [ -z "$GO_ADDR" ]; then
    echo "Failed to extract go node address. Check ../results/go-server.log"
    cat ../results/go-server.log
    kill $GO_PID 2>/dev/null
    exit 1
fi

cd ../py-libp2p-node
echo "Activating Python virtual environment..."

source venv/bin/activate
pip install trio==0.22.0 2>/dev/null || echo "Trio already installed"

echo "Starting python client to connect to: $GO_ADDR"
timeout 30 python3 main.py "$GO_ADDR" > ../results/py-client-test1.log 2>&1
PY_CLIENT_EXIT=$?

echo "Python client exit code: $PY_CLIENT_EXIT"

echo "Stopping go node..."
kill $GO_PID 2>/dev/null
wait $GO_PID 2>/dev/null

echo "Test 1 completed. Check results/py-client-test1.log"

echo "=== Testing Complete ==="
echo "Check the results/ directory for detailed logs"

echo ""
echo "=== Test Summary ==="
echo "Go server log:"
tail -5 ../results/go-server.log
echo ""
echo "Python client log:"
tail -5 ../results/py-client-test1.log

deactivate
