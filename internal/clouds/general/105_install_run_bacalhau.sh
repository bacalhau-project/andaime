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

start_bacalhau() {
    log "Starting Bacalhau..."
    
    # Construct LABELS from Node Info
    LABELS="EC2_INSTANCE_FAMILY=${EC2_INSTANCE_FAMILY:-unknown},EC2_VCPU_COUNT=${EC2_VCPU_COUNT:-unknown},EC2_MEMORY_GB=${EC2_MEMORY_GB:-unknown},EC2_DISK_GB=${EC2_DISK_GB:-unknown},ORCHESTRATORS=${ORCHESTRATORS},HOSTNAME=${HOSTNAME},IP=${IP}"
    
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