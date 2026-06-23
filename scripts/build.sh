#!/bin/bash
set -e
cd "$(dirname "$0")/.."

echo "Building cbridge..."
(cd cbridge && bash build.sh)

echo "Building Wails app..."
wails build --clean -tags webkit2_41

echo "Copying runtime libs to build/bin..."
cp -f libsteam_bridge.so build/bin/
cp -f libsteam_api.so build/bin/

echo "Done."
