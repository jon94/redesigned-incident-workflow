#!/bin/bash
# Start Temporal development server locally

set -e

echo "Starting Temporal development server..."
echo "Web UI will be available at: http://localhost:8233"
echo "gRPC endpoint: localhost:7233"
echo ""
echo "Press Ctrl+C to stop"
echo ""

temporal server start-dev \
    --ui-port 8233 \
    --db-filename temporal.db
