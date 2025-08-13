#!/bin/sh
set -exv

mkdir -p /config/qbittorrent
mkdir -p /config/flood
chown -R 3000:2001 /config
