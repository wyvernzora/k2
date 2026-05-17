# syntax=docker/dockerfile:1.7

ARG BASE_IMAGE=ubuntu:24.04
ARG KAIROS_INIT_VERSION=v0.13.0

FROM quay.io/kairos/kairos-init:${KAIROS_INIT_VERSION} AS kairos-init

FROM ${BASE_IMAGE} AS base-kairos

ARG MODEL=generic
ARG TRUSTED_BOOT=false
ARG KUBERNETES_DISTRO=k3s
ARG KUBERNETES_VERSION
ARG VERSION
ARG FIPS=no-fips
ARG OVERLAYS=

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
        if printf '%s' "${OVERLAYS}" | grep -Eq '(^|,)hardware/rpi4cb($|,)'; then \
            apt-get update; \
            apt-get install -y --no-install-recommends \
                e2fsprogs \
                fdisk \
                kmod \
                parted \
                udev \
                util-linux; \
            mkdir -p /etc/dracut.conf.d; \
            printf '%s\n' \
                'install_items+=" /usr/sbin/e2label /usr/sbin/mkfs.ext4 /usr/sbin/modprobe /usr/sbin/parted /usr/sbin/partprobe /usr/sbin/wipefs "' \
                > /etc/dracut.conf.d/90-k2-rpi4cb-tools.conf; \
        fi; \
        ln -sf "/boot/vmlinuz-${raspi_kernel}" /boot/vmlinuz; \
        dracut --force /boot/initrd "${raspi_kernel}"; \
    fi; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /usr/src/linux-headers-* /usr/src/linux-raspi-headers-*

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
