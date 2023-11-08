#!/bin/sh -e

# Use this file to specify PCI devices that are to be passed-through to VMs
# e.g.
#
#  echo "vfio-pci" > /sys/devices/pci0000:00/0000:00:01.0/0000:01:00.0/driver_override

modprobe -i vfio-pci
