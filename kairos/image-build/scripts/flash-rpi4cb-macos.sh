#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KAIROS_DIR="${KAIROS_DIR:-$(cd "${SCRIPT_DIR}/../.." && pwd)}"
ARTIFACT_ROOT="${ARTIFACT_ROOT:-${KAIROS_DIR}/artifacts}"

TARGET="ubuntu-24.04-standard-arm64-rpi4cb-k3s-base"
EMMC_DISK=""
NVME_DISK=""
ZERO_NVME="false"
SKIP_VERIFY="false"

log() {
    printf '[INFO] %s\n' "$*"
}

warn() {
    printf '[WARN] %s\n' "$*" >&2
}

success_banner() {
    local verify_status="$1"

    printf '\n'
    printf '        _________________________________\n'
    printf '       /                                 \\\n'
    printf '      |      FLASH COMPLETE              |\n'
    printf '      |      %-27s|\n' "${verify_status}"
    printf '      |      Safe to unplug/reboot       |\n'
    printf '       \\_________________________________/\n'
    printf '\n'
}

failure_banner() {
    local status="$1"

    cat >&2 <<EOF

        _________________________________
       /                                 \\
      |      FLASH FAILED                 |
      |      exit status: ${status}                |
      |      Check the last error above    |
       \\_________________________________/

EOF
}

usage() {
    cat <<EOF
Usage: $0 --emmc diskN [--nvme diskM] [--zero-nvme] [--skip-verify]

Flashes the ${TARGET} raw image to a CM4 eMMC exposed through rpiboot on macOS.

Examples:
  $0 --emmc disk5
  $0 --emmc disk5 --nvme disk6 --zero-nvme

Options:
  --emmc diskN     Required. The 31GB CM4 eMMC disk identifier from diskutil.
  --nvme diskM     Optional. The 256GB NVMe disk identifier from diskutil.
  --zero-nvme      Require --nvme. Zero the first 64 MiB of NVMe metadata.
  --skip-verify    Skip byte-for-byte eMMC prefix hash verification.
  -h, --help       Show this help.
EOF
}

die() {
    echo "[ERROR] $*" >&2
    exit 1
}

require_cmd() {
    local cmd="$1"

    if ! command -v "${cmd}" >/dev/null 2>&1; then
        die "missing required command: ${cmd}"
    fi
}

artifact_dir() {
    local target="$1"

    echo "${ARTIFACT_ROOT}/${target}"
}

disk_path() {
    local disk="$1"
    echo "/dev/${disk}"
}

raw_disk_path() {
    local disk="$1"
    echo "/dev/r${disk}"
}

diskutil_value() {
    local disk="$1"
    local key="$2"

    diskutil info -plist "$(disk_path "${disk}")" |
        plutil -extract "${key}" raw -o - -
}

validate_target_disk() {
    local disk="$1"
    local role="$2"
    local whole internal size_bytes max_bytes

    log "Validating ${role} target $(disk_path "${disk}")"
    whole="$(diskutil_value "${disk}" WholeDisk)"
    internal="$(diskutil_value "${disk}" Internal)"
    size_bytes="$(diskutil_value "${disk}" TotalSize)"
    max_bytes=$(( 512 * 1024 * 1024 * 1024 ))

    if [ "${whole}" != "true" ]; then
        die "${role} $(disk_path "${disk}") is not a whole disk"
    fi

    if [ "${internal}" != "false" ]; then
        die "${role} $(disk_path "${disk}") is not marked external by diskutil"
    fi

    if [ "${size_bytes}" -gt "${max_bytes}" ]; then
        die "${role} $(disk_path "${disk}") is larger than 512GB (${size_bytes} bytes)"
    fi

    if ! diskutil list "$(disk_path "${disk}")" | head -n 1 | grep -F "(external, physical)" >/dev/null; then
        die "${role} $(disk_path "${disk}") is not marked as (external, physical) in diskutil list"
    fi

    log "${role} target accepted: $(disk_path "${disk}") (${size_bytes} bytes)"
}

require_macos() {
    if [ "$(uname -s)" != "Darwin" ]; then
        die "this helper is macOS-only"
    fi
}

confirm() {
    local raw_file="$1"

    echo
    echo "About to flash:"
    echo "  image: ${raw_file}"
    echo "  eMMC:  $(disk_path "${EMMC_DISK}")"
    if [ -n "${NVME_DISK}" ]; then
        echo "  NVMe:  $(disk_path "${NVME_DISK}")"
        echo "  zero NVMe boot metadata: ${ZERO_NVME}"
    fi
    echo
    diskutil list "$(disk_path "${EMMC_DISK}")" || true
    if [ -n "${NVME_DISK}" ]; then
        diskutil list "$(disk_path "${NVME_DISK}")" || true
    fi
    echo
    read -r -p "Type FLASH to write ${raw_file} to $(disk_path "${EMMC_DISK}"): " answer
    if [ "${answer}" != "FLASH" ]; then
        die "confirmation failed; aborting"
    fi
}

verify_prefix_hash() {
    local raw_file="$1"
    local bytes block_size block_count image_hash disk_hash

    log "Verifying eMMC prefix hash against image"
    bytes="$(stat -f %z "${raw_file}")"
    if [ $(( bytes % 1048576 )) -eq 0 ]; then
        block_size="1m"
        block_count=$(( bytes / 1048576 ))
    elif [ $(( bytes % 512 )) -eq 0 ]; then
        block_size="512"
        block_count=$(( bytes / 512 ))
    else
        die "raw image size ${bytes} is not sector-aligned; cannot verify disk prefix safely"
    fi

    echo "Image:"
    image_hash="$(shasum -a 256 "${raw_file}" | awk '{print $1}')"
    echo "${image_hash}  ${raw_file}"

    echo "eMMC prefix:"
    disk_hash="$(sudo dd if="$(raw_disk_path "${EMMC_DISK}")" bs="${block_size}" count="${block_count}" status=progress | shasum -a 256 | awk '{print $1}')"
    echo "${disk_hash}  -"

    if [ "${image_hash}" != "${disk_hash}" ]; then
        die "verification failed: eMMC prefix hash does not match image"
    fi

    log "Verification passed"
}

main() {
    local raw_file
    local verify_status="eMMC image verified"

    trap 'failure_banner "$?"' ERR

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --emmc)
                EMMC_DISK="${2:-}"
                shift 2
                ;;
            --nvme)
                NVME_DISK="${2:-}"
                shift 2
                ;;
            --zero-nvme)
                ZERO_NVME="true"
                shift
                ;;
            --skip-verify)
                SKIP_VERIFY="true"
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                die "unknown argument: $1"
                ;;
        esac
    done

    require_macos
    require_cmd diskutil
    require_cmd find
    require_cmd plutil
    require_cmd shasum

    log "Starting rpi4cb flash helper"

    if [ -z "${EMMC_DISK}" ]; then
        usage
        exit 1
    fi
    if [ "${ZERO_NVME}" = "true" ] && [ -z "${NVME_DISK}" ]; then
        die "--zero-nvme requires --nvme diskM"
    fi

    validate_target_disk "${EMMC_DISK}" "eMMC"
    if [ -n "${NVME_DISK}" ]; then
        validate_target_disk "${NVME_DISK}" "NVMe"
    fi

    raw_file="$(find "$(artifact_dir "${TARGET}")" -maxdepth 1 -type f -name '*.raw' -print -quit)"
    if [ -z "${raw_file}" ] || [ ! -f "${raw_file}" ]; then
        die "raw image not found under $(artifact_dir "${TARGET}"); build it first with: earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=${TARGET}"
    fi
    log "Using raw image: ${raw_file}"
    log "Raw image size: $(stat -f %z "${raw_file}") bytes"

    confirm "${raw_file}"

    log "Unmounting eMMC $(disk_path "${EMMC_DISK}")"
    diskutil unmountDisk "$(disk_path "${EMMC_DISK}")" || true

    log "Flashing eMMC. This can take a few minutes."
    sudo dd if="${raw_file}" of="$(raw_disk_path "${EMMC_DISK}")" bs=4m status=progress
    log "Syncing eMMC writes"
    sync

    if [ "${SKIP_VERIFY}" != "true" ]; then
        verify_prefix_hash "${raw_file}"
    else
        warn "Skipping eMMC verification because --skip-verify was set"
        verify_status="eMMC verification skipped"
    fi

    if [ -n "${NVME_DISK}" ] && [ "${ZERO_NVME}" = "true" ]; then
        log "Unmounting NVMe $(disk_path "${NVME_DISK}")"
        diskutil unmountDisk "$(disk_path "${NVME_DISK}")" || true
        log "Zeroing first 64 MiB of NVMe"
        sudo dd if=/dev/zero of="$(raw_disk_path "${NVME_DISK}")" bs=1m count=64 status=progress
        log "Syncing NVMe metadata wipe"
        sync
    elif [ -n "${NVME_DISK}" ]; then
        log "Leaving NVMe unchanged because --zero-nvme was not set"
    fi

    log "Ejecting disks"
    diskutil eject "$(disk_path "${EMMC_DISK}")" || true
    if [ -n "${NVME_DISK}" ]; then
        diskutil eject "$(disk_path "${NVME_DISK}")" || true
    fi

    trap - ERR
    success_banner "${verify_status}"
}

main "$@"
