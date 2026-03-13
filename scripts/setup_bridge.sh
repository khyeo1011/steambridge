#!/bin/bash

PEER_ID=$1
INTERFACE="steambridge0"
IP_ADDR="10.1.0.11/24"
PROJECT_ROOT="$(dirname "$0")/.."

if [ -z "$PEER_ID" ]; then
    echo "Usage: sudo ./scripts/setup_bridge.sh <REMOTE_STEAM_ID>"
    exit 1
fi

echo "Starting SteamBridge for peer $PEER_ID..."
cd "$PROJECT_ROOT" || exit
sudo go run cmd/steambridge/main.go --iface "$INTERFACE" --peer "$PEER_ID" &
BRIDGE_PID=$!

echo "Waiting for $INTERFACE to initialize..."
MAX_RETRIES=10
COUNT=0
while ! ip link show "$INTERFACE" > /dev/null 2>&1; do
    sleep 0.5
    ((COUNT++))
    if [ $COUNT -ge $MAX_RETRIES ]; then
        echo "Error: TAP interface was never created."
        kill $BRIDGE_PID
        exit 1
    fi
done

echo "Configuring $INTERFACE with IP $IP_ADDR..."
sudo ip addr add "$IP_ADDR" dev "$INTERFACE"
sleep 0.5
sudo ip link set dev "$INTERFACE" up

echo "Bridge is live. Logs follow:"
wait $BRIDGE_PID