# syntax=docker/dockerfile:1.25

ARG BASE_IMAGE=ubuntu:26.04
ARG KAIROS_INIT_VERSION=v0.15.0

FROM quay.io/kairos/kairos-init:${KAIROS_INIT_VERSION} AS kairos-init

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS k2-node-agent

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/kairos/node-agent
COPY kairos/node-agent/go.mod ./
COPY kairos/node-agent/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /out/k2-node-agent ./cmd/k2-node-agent

FROM ${BASE_IMAGE} AS base-kairos

ARG MODEL=generic
ARG TRUSTED_BOOT=false
ARG KUBERNETES_DISTRO=k3s
ARG KUBERNETES_VERSION
ARG KAIROS_VERSION
ARG VERSION
ARG FIPS=no-fips
ARG OVERLAYS=
ARG TARGET_NAME
ARG FLAVOR
ARG ROLE
ARG ARCH
ARG PLATFORM
ARG HARDWARE
ARG IMAGE_REVISION
ARG APT_PACKAGES=
ARG APT_PURGE_PACKAGES=
ARG DRACUT_INSTALL_ITEMS=
ARG POST_INSTALL_ACTIONS=

COPY --from=k2-node-agent /out/k2-node-agent /usr/sbin/k2-node-agent

RUN --mount=type=bind,from=kairos-init,src=/kairos-init,dst=/kairos-init \
    set -eux; \
    if [ -n "${KUBERNETES_DISTRO}" ]; then \
        K8S_FLAG="-p ${KUBERNETES_DISTRO}"; \
        if [ "${KUBERNETES_DISTRO}" = "k3s" ] && [ -n "${KUBERNETES_VERSION}" ]; then \
            K8S_VERSION_FLAG="--provider-k3s-version ${KUBERNETES_VERSION}"; \
        else \
            K8S_VERSION_FLAG=""; \
        fi; \
    else \
        K8S_FLAG=""; \
        K8S_VERSION_FLAG=""; \
    fi; \
    if [ "${FIPS}" = "fips" ]; then FIPS_FLAG="--fips"; else FIPS_FLAG=""; fi; \
    /kairos-init -l debug -s install -m "${MODEL}" -t "${TRUSTED_BOOT}" ${K8S_FLAG} ${K8S_VERSION_FLAG} --version "${VERSION}" ${FIPS_FLAG}; \
    /kairos-init -l debug -s init -m "${MODEL}" -t "${TRUSTED_BOOT}" ${K8S_FLAG} ${K8S_VERSION_FLAG} --version "${VERSION}" ${FIPS_FLAG}

RUN set -eux; \
    if [ "${MODEL}" = "rpi4" ]; then \
        raspi_kernel="$(find /boot -maxdepth 1 -name 'vmlinuz-*-raspi' -printf '%f\n' | sed 's/^vmlinuz-//' | sort -V | tail -n 1)"; \
        if [ -z "${raspi_kernel}" ]; then \
            echo "Unable to find Raspberry Pi kernel under /boot" >&2; \
            exit 1; \
        fi; \
        purge_packages="$(dpkg-query -W -f='${binary:Package}\n' \
            'linux-generic*' \
            'linux-image-generic*' \
            'linux-image-*-generic' \
            'linux-modules-*-generic' \
            'linux-modules-extra-*-generic' \
            'linux-headers-*' \
            'linux-raspi-headers-*' \
            2>/dev/null || true)"; \
        if [ -n "${purge_packages}" ]; then \
            apt-get purge -y ${purge_packages}; \
        fi; \
        apt-get autoremove -y; \
        ln -sf "/boot/vmlinuz-${raspi_kernel}" /boot/vmlinuz; \
    fi; \
    if [ -n "${APT_PACKAGES}" ]; then \
        apt-get update; \
        apt-get install -y --no-install-recommends ${APT_PACKAGES}; \
    fi; \
    if [ -n "${APT_PURGE_PACKAGES}" ]; then \
        purge_packages=""; \
        for package in ${APT_PURGE_PACKAGES}; do \
            installed="$(dpkg-query -W -f='${Package}' "${package}" 2>/dev/null || true)"; \
            if [ -n "${installed}" ]; then \
                purge_packages="${purge_packages} ${installed}"; \
            fi; \
        done; \
        if [ -n "${purge_packages}" ]; then \
            apt-get purge -y ${purge_packages}; \
            apt-get autoremove -y --purge; \
        fi; \
        apt-get clean; \
        rm -rf /var/lib/apt/lists/*; \
    fi; \
    if [ -n "${DRACUT_INSTALL_ITEMS}" ]; then \
        mkdir -p /etc/dracut.conf.d; \
        printf 'install_items+=" %s "\n' "${DRACUT_INSTALL_ITEMS}" > /etc/dracut.conf.d/90-k2-install-items.conf; \
    fi; \
    needs_initrd_rebuild=false; \
    for action in ${POST_INSTALL_ACTIONS}; do \
        case "${action}" in \
            zfs-hostid) [ -f /etc/hostid ] || zgenhostid ;; \
            disable-motd) if [ -d /etc/update-motd.d ]; then chmod -x /etc/update-motd.d/* || true; fi ;; \
            use-virtual-kernel) \
                apt-get update; \
                apt-get install -y --no-install-recommends linux-image-virtual; \
                # linux-image-virtual Depends on the SAME versioned
                # linux-image-<ver>-generic binary; only the METAS are
                # purged below. linux-main-modules-zfs-* (zfs.ko) hangs off
                # the generic meta alone, so pin it before the purge or
                # autoremove sweeps it and the storage role dies at boot.
                zfs_modules="$(dpkg-query -W -f='${binary:Package}\n' 'linux-main-modules-zfs-*' 2>/dev/null || true)"; \
                if [ -n "${zfs_modules}" ]; then \
                    apt-mark manual ${zfs_modules}; \
                fi; \
                purge_packages=""; \
                for package in \
                    linux-image-generic \
                    linux-generic \
                    linux-firmware \
                    linux-firmware-amd-graphics \
                    linux-firmware-amd-misc \
                    linux-firmware-broadcom-wireless \
                    linux-firmware-intel-graphics \
                    linux-firmware-intel-misc \
                    linux-firmware-intel-wireless \
                    linux-firmware-marvell-prestera \
                    linux-firmware-marvell-wireless \
                    linux-firmware-mediatek \
                    linux-firmware-mellanox-spectrum \
                    linux-firmware-misc \
                    linux-firmware-netronome \
                    linux-firmware-nvidia-graphics \
                    linux-firmware-qlogic \
                    linux-firmware-qualcomm-graphics \
                    linux-firmware-qualcomm-misc \
                    linux-firmware-qualcomm-wireless \
                    linux-firmware-realtek \
                    linux-firmware-snapdragon \
                    linux-firmware-raspi2; do \
                    installed="$(dpkg-query -W -f='${binary:Package}\n' "${package}" 2>/dev/null || true)"; \
                    if [ -n "${installed}" ]; then \
                        purge_packages="${purge_packages} ${installed}"; \
                    fi; \
                done; \
                if [ -n "${purge_packages}" ]; then \
                    apt-get purge -y ${purge_packages}; \
                fi; \
                apt-get autoremove -y --purge; \
                apt-get clean; \
                rm -rf /var/lib/apt/lists/*; \
                # linux-modules ships ~105MB of per-kernel firmware under
                # /usr/lib/firmware/<kver> that apt cannot drop without
                # dropping the drivers. A VM loads zero firmware files, so
                # remove the tree outright; dpkg bookkeeping divergence is
                # harmless on immutable images (every rebuild re-runs this).
                rm -rf /usr/lib/firmware; \
                kernel_paths="$(find /boot -maxdepth 1 -type f -name 'vmlinuz-*' -printf '%p\n' | sort -V)"; \
                kernel_count="$(printf '%s\n' "${kernel_paths}" | sed '/^$/d' | wc -l)"; \
                if [ "${kernel_count}" -ne 1 ]; then \
                    echo "Expected exactly one vmlinuz-* under /boot after virtual kernel swap, found ${kernel_count}: ${kernel_paths}" >&2; \
                    exit 1; \
                fi; \
                kernel_path="$(printf '%s\n' "${kernel_paths}" | sed -n '1p')"; \
                kernel_version="$(basename "${kernel_path}" | sed 's/^vmlinuz-//')"; \
                # The binary kernel keeps its -generic name under the
                # virtual meta; assert the meta is what anchors it.
                if ! dpkg -s linux-image-virtual >/dev/null 2>&1; then \
                    echo "linux-image-virtual is not installed after the swap" >&2; \
                    exit 1; \
                fi; \
                modules_dir="/usr/lib/modules/${kernel_version}"; \
                if [ ! -d "${modules_dir}" ]; then \
                    echo "Missing module directory for virtual kernel: ${modules_dir}" >&2; \
                    exit 1; \
                fi; \
                for module in zfs iscsi_tcp target_core_mod; do \
                    if ! find "${modules_dir}" -name "${module}.ko*" -print -quit | grep -q .; then \
                        echo "Missing required module ${module}.ko* under ${modules_dir}" >&2; \
                        exit 1; \
                    fi; \
                done; \
                if [ -d /usr/lib/firmware ]; then \
                    firmware_mb="$(du -sm /usr/lib/firmware | awk '{print $1}')"; \
                    if [ "${firmware_mb}" -ge 5 ]; then \
                        echo "Expected /usr/lib/firmware to be absent or under 5MB after virtual kernel swap, found ${firmware_mb}MB" >&2; \
                        exit 1; \
                    fi; \
                fi; \
                needs_initrd_rebuild=true ;; \
            *) echo "Unknown post-install action: ${action}" >&2; exit 1 ;; \
        esac; \
    done; \
    if [ "${MODEL}" = "rpi4" ] || [ -n "${DRACUT_INSTALL_ITEMS}" ] || [ "${needs_initrd_rebuild}" = "true" ]; then \
        kernel="$(readlink -f /boot/vmlinuz 2>/dev/null | sed 's|.*/vmlinuz-||')"; \
        if [ -z "${kernel}" ]; then \
            kernel="$(find /boot -maxdepth 1 -name 'vmlinuz-*' -printf '%f\n' | sed 's/^vmlinuz-//' | sort -V | tail -n 1)"; \
        fi; \
        if [ -z "${kernel}" ]; then \
            echo "Unable to determine kernel under /boot" >&2; \
            exit 1; \
        fi; \
        dracut --force /boot/initrd "${kernel}"; \
    fi; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /usr/src/linux-headers-* /usr/src/linux-raspi-headers-*

RUN set -eux; \
    mkdir -p /usr/share/k2/image-build; \
    { \
        printf 'target: %s\n' "${TARGET_NAME}"; \
        printf 'flavor: %s\n' "${FLAVOR}"; \
        printf 'role: %s\n' "${ROLE}"; \
        printf 'arch: %s\n' "${ARCH}"; \
        printf 'platform: %s\n' "${PLATFORM}"; \
        printf 'hardware: %s\n' "${HARDWARE}"; \
        printf 'kairosModel: %s\n' "${MODEL}"; \
        printf 'kubernetesDistro: %s\n' "${KUBERNETES_DISTRO}"; \
        printf 'kubernetesVersion: %s\n' "${KUBERNETES_VERSION}"; \
        printf 'kairosVersion: %s\n' "${KAIROS_VERSION}"; \
        printf 'imageRevision: %s\n' "${IMAGE_REVISION}"; \
    } > /usr/share/k2/image-build/metadata.yaml

# ----------------------------------------------------------------
# k2-rescue: UART-only emergency recovery user.
#
# Baked into both COS_ACTIVE and COS_RECOVERY rootfs so it survives
# `kairos-agent reset` (incl. `--reset-oem`) and any provisioner-
# hardening that disables /oem cloud-configs. Required because
# COS_PERSISTENT (operator SSH key, /home/kairos) gets wiped by
# reset; without an OS-rootfs-resident fallback, a failed reset
# bricks the node to reflash-only.
#
# Network access denied unconditionally (DenyUsers in sshd_config.d
# below): use is UART-at-the-rack only. On active boot, the user is
# additionally locked + de-sudo'd by a yip cloud-config stage
# shipped by the base overlay; effective access is recovery-
# boot only.
#
# Cleartext password lives on a laser-engraved metal plate kept with
# the operator's 2FA recovery codes. Not in any password manager,
# not anywhere else digital. Rotation = edit the hash literal below,
# rebuild image, re-engrave plate.
# ----------------------------------------------------------------
RUN set -eux; \
    useradd --create-home --shell /bin/bash k2-rescue; \
    echo 'k2-rescue:$y$j9T$5oZ2TfT8.Q38aVPqk2jPT/$SLyChcYpT3rj1KjRU4giE2MLifEXwiW1jg/bv5J/a71' | chpasswd -e; \
    printf '%s\n' 'k2-rescue ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/99-k2-rescue; \
    chmod 0440 /etc/sudoers.d/99-k2-rescue; \
    mkdir -p /etc/ssh/sshd_config.d; \
    printf '%s\n' 'DenyUsers k2-rescue' > /etc/ssh/sshd_config.d/99-k2-rescue-block.conf

COPY kairos/overlays /k2-overlays

RUN set -eux; \
    if [ -n "${OVERLAYS}" ]; then \
        old_ifs="${IFS}"; \
        IFS=','; \
        for overlay in ${OVERLAYS}; do \
            overlay_dir="/k2-overlays/${overlay}"; \
            if [ ! -d "${overlay_dir}" ]; then \
                echo "Missing overlay: ${overlay}" >&2; \
                exit 1; \
            fi; \
            oci_dir="${overlay_dir}/oci"; \
            if [ -d "${oci_dir}" ] && [ -n "$(find "${oci_dir}" -mindepth 1 -print -quit)" ]; then \
                tar --exclude .gitkeep -C "${oci_dir}" -cf - . | tar -C / -xf -; \
            fi; \
        done; \
        IFS="${old_ifs}"; \
    fi; \
    rm -rf /k2-overlays
