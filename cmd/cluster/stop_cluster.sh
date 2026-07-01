#!/bin/bash

PIDFILE=`pwd`/cluster.pids
TIMEOUT=60  # seconds to wait for graceful shutdown before SIGKILL

if [ -f "$PIDFILE" ]; then
    pids=()
    while IFS= read -r p; do pids+=("$p"); done < "$PIDFILE"
    if [ ${#pids[@]} -eq 0 ]; then
        echo "PID file is empty, nothing to stop."
        rm -f "$PIDFILE"
        exit 0
    fi
    echo "Using PID file: $PIDFILE"
else
    # Fallback: find cluster processes by their --cluster_config argument.
    # Note: if multiple clusters with different configs are running on this
    # machine, this will stop all of them.
    echo "PID file not found, falling back to ps lookup..."
    pids=()
    while IFS= read -r p; do pids+=("$p"); done < <(ps -ef | grep -- '--cluster_config' | grep -v grep | awk '{print $2}')
    if [ ${#pids[@]} -eq 0 ]; then
        echo "No cluster processes found."
        exit 0
    fi
    echo "Found pids via ps: ${pids[*]}"
fi

# Send SIGTERM to all processes so they can flush state and close DB cleanly.
echo "Sending SIGTERM to cluster processes..."
for pid in "${pids[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
        kill -TERM "$pid"
        echo "  SIGTERM -> pid $pid"
    else
        echo "  pid $pid not running, skipping"
    fi
done

# Wait up to TIMEOUT seconds for all processes to exit, printing status every 10s.
deadline=$(($(date +%s) + TIMEOUT))
remaining=("${pids[@]}")
last_print=0
while [ ${#remaining[@]} -gt 0 ] && [ $(date +%s) -lt $deadline ]; do
    sleep 1
    now=$(date +%s)
    still_running=()
    for pid in "${remaining[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            still_running+=("$pid")
        fi
    done
    remaining=("${still_running[@]}")
    elapsed=$((now - (deadline - TIMEOUT)))
    if [ $((elapsed - last_print)) -ge 10 ] && [ ${#remaining[@]} -gt 0 ]; then
        echo "  [${elapsed}s] still waiting on pids: ${remaining[*]}"
        last_print=$elapsed
    fi
done

# Force-kill any processes that did not exit in time.
if [ ${#remaining[@]} -gt 0 ]; then
    echo "The following pids did not exit within ${TIMEOUT}s, sending SIGKILL:"
    for pid in "${remaining[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            echo "  SIGKILL -> pid $pid"
            kill -9 "$pid"
        fi
    done
else
    echo "All cluster processes stopped cleanly."
fi

rm -f "$PIDFILE"
