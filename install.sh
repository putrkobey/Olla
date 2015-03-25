#!/usr/bin/env bash
#########################
# Universal Installer Script v2.0.0
# ---------------------------------
# Simple, lightweight & reusable
#
# <github.com/thushan>
#########################
set -e  # Exit on error

# CONFIGURATION
repo="olla"      # GitHub repo name
name="olla"      # Name of the binary/project
exec="olla"      # Executable name

# Detect if we can use colors, sometimes running in non-tty environments
if [[ -t 1 ]] && [[ -n "$(tput colors 2>/dev/null)" ]] && [[ "$(tput colors)" -ge 8 ]]; then
    CRed=$(tput setaf 1)
    CBlue=$(tput setaf 4)
    CCyan=$(tput setaf 6)
    CGreen=$(tput setaf 2)
    CYellow=$(tput setaf 3)
    BOLD=$(tput bold)
    ENDMARKER=$(tput sgr0)
else
    # No color support
    CRed=""
    CBlue=""
    CCyan=""
    CGreen=""
    CYellow=""
    BOLD=""
    ENDMARKER=""
fi

# Runtime configuration
version="latest"
install_dir=""
show_progress=true
local_file=""

# Allow environment variable overrides for testing
# Example: INSTALLER_REPO=smash ./install.sh
repo="${INSTALLER_REPO:-$repo}"
name="${INSTALLER_NAME:-$name}"
exec="${INSTALLER_EXEC:-$exec}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            echo "Universal Installer Script v2.0.0"
            echo "github.com/thushan/${repo}"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -h, --help              Show this help message"
            echo "  -v, --version VERSION   Install specific version (default: latest)"
            echo "  -d, --dir DIRECTORY     Install to specific directory"
            echo "  -q, --quiet             Quiet mode (no progress)"
            echo "  --no-color              Disable colored output"
            echo "  --local FILE            Use local archive instead of downloading"
            echo ""
            echo "Examples:"
            echo "  $0                      # Install latest version"
            echo "  $0 -v v1.2.3            # Install specific version"
            echo "  $0 -d /usr/local/bin    # Install to specific directory"
            exit 0
            ;;
        -v|--version)
            version="$2"
            shift 2
            ;;
        -d|--dir)
            install_dir="$2"
            shift 2
            ;;
        -q|--quiet)
            show_progress=false
            shift
            ;;
        --no-color)
            CRed=""
            CBlue=""
            CCyan=""
            CGreen=""
            CYellow=""
            BOLD=""
            ENDMARKER=""
            shift
            ;;
        --local)
            local_file="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo "----------------------"
echo "${CGreen}${name} Auto-Installer v2.0.0${ENDMARKER}"
echo "${CBlue} <github.com/thushan/${repo}>${ENDMARKER}"
echo "----------------------"

# Cleanup function
cleanup() {
    if [[ -n "$temp_file" ]] && [[ -f "$temp_file" ]]; then
        rm -f "$temp_file"
    fi
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

function fatal() {
    echo ""
    echo "${BOLD}${CRed}FATAL:${ENDMARKER} $1${ENDMARKER}" >&2
    echo ""
    cleanup
    exit 1
}

# Check for required commands
for cmd in curl grep sed; do
    if ! command -v "$cmd" &> /dev/null; then
        fatal "Required command '$cmd' not found. Please install it and try again."
    fi
done
