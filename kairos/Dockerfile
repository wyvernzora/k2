# syntax=docker/dockerfile:1.25

ARG BASE_IMAGE=ubuntu:26.04
ARG KAIROS_INIT_VERSION=v0.13.1

FROM quay.io/kairos/kairos-init:${KAIROS_INIT_VERSION} AS kairos-init

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS k2-node-init

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/kairos/node-init
COPY kairos/node-init/go.mod ./
COPY kairos/node-init/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/k2-node-init ./cmd/k2-node-init

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
ARG FLAVOR_RELEASE
ARG VARIANT
ARG ARCH
ARG PLATFORM
ARG HARDWARE
ARG IMAGE_REVISION

COPY --from=k2-node-init /out/k2-node-init /usr/local/bin/k2-node-init

RUN --mount=type=bind,from=kairos-init,src=/kairos-init,dst=/kairos-init \
    set -eux; \
    if [ -n "${KUBERNETES_DISTRO}" ]; then \
        K8S_FLAG="-p ${KUBERNETES_DISTRO}"; \
        if [ "${KUBERNETES_DISTRO}" = "k0s" ] && [ -n "${KUBERNETES_VERSION}" ]; then \
            K8S_VERSION_FLAG="--provider-k0s-version ${KUBERNETES_VERSION}"; \
        elif [ "${KUBERNETES_DISTRO}" = "k3s" ] && [ -n "${KUBERNETES_VERSION}" ]; then \
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
    needs_persistent_tools="0"; \
    needs_qemu_agent="0"; \
    if printf '%s' "${OVERLAYS}" | grep -Eq '(^|,)hardware/(qemu|rpi4cb)($|,)'; then \
        needs_persistent_tools="1"; \
    fi; \
    if printf '%s' "${OVERLAYS}" | grep -Eq '(^|,)hardware/qemu($|,)'; then \
        needs_qemu_agent="1"; \
    fi; \
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
    if [ "${needs_persistent_tools}" = "1" ] || [ "${needs_qemu_agent}" = "1" ]; then \
        apt-get update; \
    fi; \
    if [ "${needs_persistent_tools}" = "1" ]; then \
        apt-get install -y --no-install-recommends \
            e2fsprogs \
            fdisk \
            kmod \
            parted \
            udev \
            util-linux; \
        mkdir -p /etc/dracut.conf.d; \
        printf '%s\n' \
            'install_items+=" /usr/local/bin/k2-node-init /usr/sbin/blkid /usr/bin/findmnt /usr/bin/lsblk /usr/bin/mount /usr/bin/readlink /usr/bin/sh /usr/bin/tar /usr/bin/umount /usr/sbin/e2label /usr/sbin/mkfs.ext4 /usr/sbin/modprobe /usr/sbin/parted /usr/sbin/partprobe /usr/sbin/resize2fs /usr/sbin/wipefs "' \
            > /etc/dracut.conf.d/90-k2-persistent-tools.conf; \
    fi; \
    if [ "${needs_qemu_agent}" = "1" ]; then \
        apt-get install -y --no-install-recommends qemu-guest-agent; \
    fi; \
    if [ "${MODEL}" = "rpi4" ] || [ "${needs_persistent_tools}" = "1" ]; then \
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
        printf 'flavorRelease: "%s"\n' "${FLAVOR_RELEASE}"; \
        printf 'variant: %s\n' "${VARIANT}"; \
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
# shipped alongside the k3s overlay; effective access is recovery-
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
