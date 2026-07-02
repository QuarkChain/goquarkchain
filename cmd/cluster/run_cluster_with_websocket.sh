#!/bin/bash

# Thin wrapper: starts a cluster with WebSocket enabled on all slave nodes.
exec "$(dirname "$0")/run_cluster.sh" "$1" "--ws --ws_host 0.0.0.0"
