#!/usr/bin/env bash

set -euo pipefail

RUN_BACALHAU_SH="/root/run_bacalhau.sh"

cat << 'EOF' > "${RUN_BACALHAU_SH}"
#!/usr/bin/env bash

set -euo pipefail

LOG_FILE="/var/log/bacalhau_start.log"

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}


# Source the configuration file
if [ -f /etc/node-config ]; then
    # shellcheck disable=SC1091
    source /etc/node-config
else
    log "Error: /etc/node-config file not found."
    exit 1
fi

check_orchestrators() {
    if [ -z "${ORCHESTRATORS:-}" ]; then
        log "Error: ORCHESTRATORS environment variable is not set."
        exit 1
    fi
}

get_current_labels() {
    local config_json
    config_json=$(bacalhau config list --output json)
    if [ -z "$config_json" ]; then
        log "Error: Failed to get Bacalhau configuration"
        return 1
    fi

    local labels
    labels=$(echo "$config_json" | jq -r '.[] | select(.Key == "Labels") | .Value | to_entries | map("\(.key)=\(.value)") | join(",")')
    echo "$labels"
}

start_bacalhau() {
    log "Starting Bacalhau..."
    
    # Initialize labels with current labels from Bacalhau config
    LABELS=$(get_current_labels)
    if [ $? -ne 0 ]; then
        log "Failed to get current labels. Proceeding with empty labels."
        LABELS=""
    fi

    # Read each line from node-config
    while IFS= read -r line
    do  
        # Skip empty lines and lines starting with TOKEN
        [[ -z "$line" || "$line" =~ ^TOKEN ]] && continue
    
        # Extract variable name and value
        var_name=$(echo "$line" | cut -d'=' -f1)
        var_value=${!var_name}
    
        # Remove any quotes from the value
        var_value=$(echo "$var_value" | tr -d '"')
    
        # Append to LABELS string
        LABELS="${LABELS:+$LABELS,}${var_name}=${var_value}"
    done < /etc/node-config

    # Print the labels for verification
    echo "Constructed Labels:"
    echo "$LABELS"
    
    if [ -n "${TOKEN:-}" ]; then
        ORCHESTRATORS="${TOKEN}@${ORCHESTRATORS}"
    fi

    # Start Bacalhau
    /usr/local/bin/bacalhau serve \
        --node-type "${NODE_TYPE}" \
        --orchestrators "${ORCHESTRATORS}" \
        --labels "${LABELS}" \
        >> "${LOG_FILE}" 2>&1 &
    
    local PID=$!
    log "Bacalhau worker node started with PID ${PID}"
    log "Labels: ${LABELS}"
}

stop_bacalhau() {
    log "Stopping Bacalhau worker node..."
    pkill -f "bacalhau serve" || true
    log "Bacalhau worker node stopped"
}

# Main execution
main() {
    local cmd="${1:-}"
    
    case "${cmd}" in
        start)
            check_orchestrators
            start_bacalhau
            ;;
        stop)
            stop_bacalhau
            ;;
        restart)
            stop_bacalhau
            sleep 2
            check_orchestrators
            start_bacalhau
            ;;
        *)
            echo "Usage: $0 {start|stop|restart}"
            exit 1
            ;;
    esac
}

main "$@"
EOF

chmod +x "${RUN_BACALHAU_SH}"

echo "Bacalhau service script has been created at ${RUN_BACALHAU_SH}"