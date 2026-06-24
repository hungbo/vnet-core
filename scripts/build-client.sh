#!/bin/bash
set -e

VERSION=${1:-dev}
echo "=== Building VNET Client + Agent v$VERSION ==="

echo "-> Building Agent..."
cd "$(dirname "$0")/../agent"
go build -ldflags="-s -w -H windowsgui" -o vnet-agent.exe ./main.go

echo "-> Building Client (Wails)..."
cd ../client
wails build -platform windows/amd64

echo "-> Packing..."
mkdir -p dist
cp build/bin/VNET.exe dist/
cp ../agent/vnet-agent.exe dist/

cd dist
cat > install.bat << 'EOF'
@echo off
title Cai dat VNET Agent
echo Cai dat VNET Agent...
start /wait vnet-agent.exe --install
echo Hoan tat!
pause
EOF

cd ..
zip -r "vnet-client-windows-${VERSION}.zip" dist/*

echo "=== Done: vnet-client-windows-${VERSION}.zip ==="
ls -lh "vnet-client-windows-${VERSION}.zip"
