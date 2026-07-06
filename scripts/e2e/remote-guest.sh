#!/bin/bash
# Runs on the homelab (guest) machine via ssh, launched by run-e2e.ps1.
# Usage: bash scripts/e2e/remote-guest.sh <HOST_STEAM_ID> [--build]

set -u

HOST_STEAM_ID=$1
shift || true
BUILD=false
[ "${1:-}" = "--build" ] && BUILD=true

IFACE="steambridge0"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
PROBE_PORT="${PROBE_PORT:-5000}"

cd "$PROJECT_ROOT" || exit 1

BRIDGE_PID=""
TCP_NC_PID=""
UDP_NC_PID=""
UDP_NC_LOG="/tmp/e2e_udp.log"

cleanup() {
    [ -n "$TCP_NC_PID" ] && kill "$TCP_NC_PID" 2>/dev/null
    [ -n "$UDP_NC_PID" ] && kill "$UDP_NC_PID" 2>/dev/null
    [ -n "$BRIDGE_PID" ] && kill "$BRIDGE_PID" 2>/dev/null
    rm $UDP_NC_LOG
    sudo ip link delete "$IFACE" 2>/dev/null
}
trap cleanup SIGINT SIGTERM EXIT

if [ -z "$HOST_STEAM_ID" ]; then
    echo "Usage: bash scripts/e2e/remote-guest.sh <HOST_STEAM_ID> [--build]"
    exit 1
fi

echo "Clearing any leftover $IFACE interface..."
sudo ip link delete "$IFACE" 2>/dev/null

if [ "$BUILD" = true ]; then
    echo "Building on homelab..."
    git pull || exit 1
    (cd cbridge && bash build.sh) || exit 1
    go build ./cmd/steambridge || exit 1
fi

echo "Starting SteamBridge guest, joining host $HOST_STEAM_ID..."
sudo ./steambridge --ifaceName "$IFACE" --peer "$HOST_STEAM_ID" &
BRIDGE_PID=$!

echo "Waiting for IPAM handshake (10.8.0.x on $IFACE)..."
MAX_RETRIES=100
COUNT=0
GUEST_IP=""
while [ -z "$GUEST_IP" ]; do
    GUEST_IP=$(ip -4 addr show "$IFACE" 2>/dev/null | grep -oP '(?<=inet\s)10\.8\.0\.\d+')
    if [ -n "$GUEST_IP" ]; then
        break
    fi
    sleep 0.5
    COUNT=$((COUNT + 1))
    if [ $COUNT -ge $MAX_RETRIES ]; then
        echo "E2E_GUEST_TIMEOUT"
        exit 1
    fi
done

echo "E2E_GUEST_READY ip=$GUEST_IP"

# --- Probe listeners ---------------------------------------------------
# New scenarios that need a guest-side fixture (a listener, a packet
# counter, etc.) should be added here as one more backgrounded command,
# tracked by its own PID var and added to cleanup() above.
nc -l -k "$PROBE_PORT" >/dev/null 2>&1 &
TCP_NC_PID=$!
nc -l -u -k "$PROBE_PORT" > $UDP_NC_LOG 2>&1 &
UDP_NC_PID=$!
# ------------------------------------------------------------------------

wait "$BRIDGE_PID"
