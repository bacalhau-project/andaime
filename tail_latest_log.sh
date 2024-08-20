#!/bin/bash

# Directory containing the log files
log_dir="/tmp"

# Pattern to match
log_pattern="andaime-profile-*"

# Find the most recent log file
latest_log=$(ls -t ${log_dir}/${log_pattern} 2>/dev/null | head -n1)

if [ -z "$latest_log" ]; then
    echo "No log files found matching the pattern ${log_dir}/${log_pattern}"
    exit 1
fi

echo "Tailing the most recent log file: $latest_log"
echo "Press Ctrl+C to stop"

# Tail the most recent log file
tail -f -1000 "$latest_log"
