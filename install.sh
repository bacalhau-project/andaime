#!/usr/bin/env bash

# Ignore SC2317 for the entire file
# shellcheck disable=SC2317

# Andaime authors (c)

# Original copyright
# https://raw.githubusercontent.com/SAME-Project/SAME-installer-website/main/install_script.sh
# ------------------------------------------------------------
# Copyright (c) Microsoft Corporation and Dapr Contributors.
# Licensed under the MIT License.
# ------------------------------------------------------------

# ANDAIME CLI location
: "${ANDAIME_INSTALL_DIR:="/usr/local/bin"}"

# sudo is required to copy binary to ANDAIME_INSTALL_DIR for linux
: "${USE_SUDO:="false"}"

# Option to download pre-releases
: "${PRE_RELEASE:="false"}"

# Http request CLI
ANDAIME_HTTP_REQUEST_CLI=curl

# GitHub Organization and repo name to download release
GITHUB_ORG=bacalhau-project
GITHUB_REPO=andaime

# ANDAIME CLI filename
ANDAIME_CLI_FILENAME=andaime

ANDAIME_CLI_FILE="${ANDAIME_INSTALL_DIR}/${ANDAIME_CLI_FILENAME}"

ANDAIME_INSTALLATION_ID="BACA14A0-eeee-eeee-eeee-194519651991"

# shellcheck disable=1054,1083,1073,1056,1072,1009
ANDAIME_ENDPOINT="https://i.bacalhau.org"

# Current time in nanoseconds
START_TIME=$(date +%s)

# --- Utility Functions ---
import_command() {
    command -v "$1" > /dev/null 2>&1
}

pushEvent() {
    event_name=$1
    event_data=${2:-""}
    sent_at=$(date +"%Y-%m-%d %T")
    elapsed_time_in_seconds=$(($(date +%s) - $START_TIME))

    # If event_data is not empty, append "||" to it
    if [ -z "$event_data" ]; then
        event_data="${event_data}||"
    else
        event_data="${event_data}"
    fi

    event_name="client.install_script.${event_name}"

    # Add installation ID and install script hash to event data
    event_data="${event_data}||andaime_testing=$ANDAIME_TESTING"
    event_data="${event_data}||metrics=elapsed_time_in_seconds:${elapsed_time_in_seconds}"

    if import_command "curl"; then
        curl -s -X POST -d "{ \"uuid\": \"${ANDAIME_INSTALLATION_ID}\", \"event_name\": \"${event_name}\", \"event_data\": \"${event_data}\", \"sent_at\": \"${sent_at}\" }" -m 5 "$ANDAIME_ENDPOINT" >/dev/null &
    fi
}

command_exists() {
    import_command "$1"
}

addInstallationID() {
  # Use the value of $ANDAIME_DIR if it exists, otherwise default to ~/.andaime
  local ANDAIME_DIR="${ANDAIME_DIR:-$HOME/.andaime}"
  local CONFIG_FILE="$ANDAIME_DIR/config.yaml"

  # Create the directory if it doesn't exist
  mkdir -p "$ANDAIME_DIR" 
 
    # If the flock command exists, use it
    if command_exists "flock"; then
        # Use a lock file to prevent race conditions
        local LOCK_FILE="$ANDAIME_DIR/config.lock"
        exec 9>"$LOCK_FILE"
        flock -x 9 || { echo "Failed to acquire lock"; pushEvent "failed_to_acquire_lock"; return 1; } # Exit function, not script
    fi

  if [ ! -f "$CONFIG_FILE" ]; then
    # No config file exists; create one with the installation ID.
    echo "User:" > "$CONFIG_FILE"
    echo "  InstallationID: $ANDAIME_INSTALLATION_ID" >> "$CONFIG_FILE"
  else
    # Normalize the config file to remove extra spacing and empty lines
    awk 'NF > 0' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"

    # Read the existing installation ID (if present) into a variable
    existing_installation_id=$(awk '/^ *InstallationID:/{print $2}' "$CONFIG_FILE")

    if [ -z "$existing_installation_id" ]; then
      # If InstallationID doesn't exist, add it under the User section
      awk -v installation_id="$ANDAIME_INSTALLATION_ID" '/^ *User:/{print; print "  InstallationID:", installation_id; next} 1' "$CONFIG_FILE" > "$ANDAIME_DIR/config.tmp" && mv "$ANDAIME_DIR/config.tmp" "$CONFIG_FILE"
    else
      # Installation ID exists; replace it, preserving the rest of the config
      sed -i "s/ *InstallationID: .*/  InstallationID: $ANDAIME_INSTALLATION_ID/" "$CONFIG_FILE" 
    fi
  fi

  # Release the lock
  if command_exists "flock"; then
    flock -u 9
    rm "$LOCK_FILE"
  fi
  return 0 # Success!
}

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm" ;;
        aarch64) ARCH="arm64" ;;
        x86_64) ARCH="amd64" ;;
    esac

    OS=$(eval "echo $(uname)|tr '[:upper:]' '[:lower:]'")

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [ "$OS" == "linux" ] && [ "$ANDAIME_INSTALL_DIR" == "/usr/local/bin" ]; then
        USE_SUDO="true"
    # Darwin needs permission to copy the file to /usr/local/bin
    elif [ "$OS" == "darwin" ] && [ "$ANDAIME_INSTALL_DIR" == "/usr/local/bin" ]; then
        USE_SUDO="true"
    fi

    # If lsb_release command is available, use it to detect distro. Otherwise - have variable be NOLSB
    if [ -x "$(command -v lsb_release)" ]; then
        DISTRO=$(lsb_release -si)
    else
        DISTRO="NOLSB"
    fi
    
    pushEvent "verify_system_info" "operating_system=$OS||architecture=$ARCH||platform=$DISTRO"
}

verifySupported() {
    local supported=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64)
    local current_osarch="${OS}-${ARCH}"

    for osarch in "${supported[@]}"; do
        if [ "$osarch" == "$current_osarch" ]; then
            echo "Your system is ${OS}_${ARCH}. Your platform is $DISTRO."
            return
        fi
    done

    echo "No prebuilt binary for ${current_osarch} and platform ${DISTRO}."
    pushEvent "failed_os_arch" "operating_system=$OS||architecture=$ARCH||platform=$DISTRO"
    exit 1
}

runAsRoot() {
    local CMD="$*"

    if [ $EUID -ne 0 ] && [ "$USE_SUDO" = "true" ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

checkHttpRequestCLI() {
    if command_exists "curl" > /dev/null; then
        ANDAIME_HTTP_REQUEST_CLI=curl
    elif command_exists "wget" > /dev/null; then
        ANDAIME_HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExistingAndaime() {
    if [ -f "$ANDAIME_CLI_FILE" ]; then
        client_version=$($ANDAIME_CLI_FILE version --client --no-style --output csv --hide-header | cut -d, -f1)
        echo -e "\nANDAIME CLI is detected: $client_version"
        echo -e "Reinstalling ANDAIME CLI - ${ANDAIME_CLI_FILE}..."
        pushEvent "andaime_detected" "client_version=$client_version"
    else
        echo -e "No ANDAIME detected. Installing fresh ANDAIME CLI..."
        pushEvent "no_andaime_detected"
    fi
}

getLatestRelease() {
    # /latest ignores pre-releases, see https://docs.github.com/en/rest/releases/releases#get-the-latest-release
    local tag_regex='v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)*'
    if [ "$PRE_RELEASE" == "true" ]; then
        echo "Installing most recent pre-release version..."
        local andaimeReleaseUrl="https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases?per_page=1"
    else
        local andaimeReleaseUrl="https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases/latest"
    fi

    local latest_release=""

    if [ "$ANDAIME_HTTP_REQUEST_CLI" == "curl" ]; then
                latest_release=$(curl -s $andaimeReleaseUrl  | grep \"tag_name\" | grep -E -i "\"$tag_regex\"" | awk 'NR==1{print $2}' | sed -n 's/\"\(.*\)\",/\1/p')
    else
        latest_release=$(wget -q --header="Accept: application/json" -O - "$andaimeReleaseUrl" | grep \"tag_name\" | grep -E -i "^$tag_regex$" | awk 'NR==1{print $2}' |  sed -n 's/\"\(.*\)\",/\1/p')
    fi

    ret_val=$latest_release
}
# --- create temporary directory and cleanup when done ---
setup_tmp() {
    ANDAIME_TMP_ROOT=$(mktemp -d 2>/dev/null || mktemp -d -t 'andaime-install.XXXXXXXXXX')
    cleanup() {
        code=$?
        set +e
        trap - EXIT
        rm -rf "${ANDAIME_TMP_ROOT}"
        exit $code
    }
    trap cleanup INT EXIT
}

downloadFile() {
    LATEST_RELEASE_TAG=$1

    ANDAIME_CLI_ARTIFACT="${ANDAIME_CLI_FILENAME}_${LATEST_RELEASE_TAG}_${OS}_${ARCH}.tar.gz"
    ANDAIME_SIG_ARTIFACT="${ANDAIME_CLI_ARTIFACT}.signature.sha256"

    DOWNLOAD_BASE="https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/releases/download"

    CLI_DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${ANDAIME_CLI_ARTIFACT}"
    SIG_DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${ANDAIME_SIG_ARTIFACT}"

    CLI_TMP_FILE="$ANDAIME_TMP_ROOT/$ANDAIME_CLI_ARTIFACT"
    SIG_TMP_FILE="$ANDAIME_TMP_ROOT/$ANDAIME_SIG_ARTIFACT"

    echo "Downloading $CLI_DOWNLOAD_URL ..."
    if [ "$ANDAIME_HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SsLN "$CLI_DOWNLOAD_URL" -o "$CLI_TMP_FILE"
    else
        wget -q -O "$CLI_TMP_FILE" "$CLI_DOWNLOAD_URL"
    fi

    if [ ! -f "$CLI_TMP_FILE" ]; then
        echo "failed to download $CLI_DOWNLOAD_URL ..."
        exit 1
    fi

    pushEvent "downloaded_file"

    echo "Downloading sig file $SIG_DOWNLOAD_URL ..."
    if [ "$ANDAIME_HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SsLN "$SIG_DOWNLOAD_URL" -o "$SIG_TMP_FILE"
    else
        wget -q -O "$SIG_TMP_FILE" "$SIG_DOWNLOAD_URL"
    fi

    if [ ! -f "$SIG_TMP_FILE" ]; then
        echo "failed to download $SIG_DOWNLOAD_URL ..."
        exit 1
    fi

    pushEvent "downloaded_sig_file"
}

expandTarball() {
    echo "Extracting tarball ..."
    # echo "Extract tar file - $CLI_TMP_FILE to $ANDAIME_TMP_ROOT"
    tar xzf "$CLI_TMP_FILE" -C "$ANDAIME_TMP_ROOT"
}

verifyBin() {
    echo "NOT verifying Bin"
}

installFile() {
    local tmp_root_andaime_cli="$ANDAIME_TMP_ROOT/$ANDAIME_CLI_FILENAME"

    if [ ! -f "$tmp_root_andaime_cli" ]; then
        echo "Failed to unpack ANDAIME CLI executable."
        exit 1
    fi

    chmod o+x "$tmp_root_andaime_cli"
    if [ -f "$ANDAIME_CLI_FILE" ]; then
        runAsRoot rm -f "$ANDAIME_CLI_FILE"
    fi
    if [ ! -d "$ANDAIME_INSTALL_DIR" ]; then
        runAsRoot mkdir -p "$ANDAIME_INSTALL_DIR"
    fi
    runAsRoot cp "$tmp_root_andaime_cli" "$ANDAIME_CLI_FILE"

    if [ -f "$ANDAIME_CLI_FILE" ]; then
        echo "$ANDAIME_CLI_FILENAME installed into $ANDAIME_INSTALL_DIR successfully."
        $ANDAIME_CLI_FILE version
    else
        echo "Failed to install $ANDAIME_CLI_FILENAME"
        exit 1
    fi

    if [ ! "$(which andaime)" = "$ANDAIME_CLI_FILE" ]; then
        echo "WARNING: $ANDAIME_CLI_FILE not on PATH: $PATH" 1>&2 
    fi

    pushEvent "finished_installation"
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install ANDAIME CLI"
        echo "For support, go to https://github.com/${GITHUB_ORG}/${GITHUB_REPO}"
    fi
    pushEvent "in_trap_failed_for_any_reason" "result=$result"
    cleanup
    exit "$result"
}

cleanup() {
    if [[ -d "${ANDAIME_TMP_ROOT:-}" ]]; then
        rm -rf "$ANDAIME_TMP_ROOT"
    fi
}

# -----------------------------------------------------------------------------
# main
# -----------------------------------------------------------------------------
trap "fail_trap" EXIT

cat << EOF

Distributed Compute over Data

Andaime Repository
https://github.com/bacalhau-project/andaime

Please file an issue if you encounter any problems!
https://github.com/bacalhau-project/andaime/issues/new

===============================================================================
EOF

getSystemInfo
verifySupported
checkExistingAndaime
checkHttpRequestCLI

if [ -z "$1" ]; then
    echo "Getting the latest ANDAIME CLI..."
    getLatestRelease
else
    ret_val=v$1
fi

if [ -z "$ret_val" ]; then
    echo 1>&2 "Error getting latest release... Please file a bug here: https://github.com/bacalhau-project/andaime/issues/new"
    exit 1
fi

echo "Installing $ret_val ANDAIME CLI..."

setup_tmp
addInstallationID
downloadFile "$ret_val"
expandTarball
verifyBin
installFile

cat << EOF

  _______ _    _          _   _ _  __ __     ______  _    _
 |__   __| |  | |   /\   | \ | | |/ / \ \   / / __ \| |  | |
    | |  | |__| |  /  \  |  \| |   /   \ \_/ / |  | | |  | |
    | |  |  __  | / /\ \ |     |  <     \   /| |  | | |  | |
    | |  | |  | |/ ____ \| |\  |   \     | | | |__| | |__| |
    |_|  |_|  |_/_/    \_\_| \_|_|\_\    |_|  \____/ \____/

Thanks for installing Andaime! We're hoping to unlock an new world of more efficient computation and data, and would really love to hear from you on how we can improve.

- 🌟 Give us a star on GitHub (https://github.com/bacalhau-project/andaime)
- 👩‍💻 Request a feature! (https://github.com/bacalhau-project/andaime/issues/new)
- 🐛 File a bug! (https://github.com/bacalhau-project/andaime/issues/new)
- ✔ Join our Slack! (https://bacalhau.org/slack)
- 📖 Subscribe to our blog! (https://bacalhau.org/blog)
- ✉ Join our mailing list! (https://bacalhau.org/mailing-list)

Thanks again!
~ Team Andaime

EOF

cleanup