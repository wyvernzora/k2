#!/bin/sh
set -eu

target_state_mib=8192
sector_bytes=512
target_state_sectors=$((target_state_mib * 1024 * 1024 / sector_bytes))

usage() {
  echo "usage: $0 --mode dry-run|apply" >&2
  exit 2
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

mode=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --mode)
      [ "$#" -ge 2 ] || usage
      mode=$2
      shift 2
      ;;
    *) usage ;;
  esac
done

case "$mode" in
  dry-run) ;;
  apply)
    [ -f /run/cos/recovery_mode ] || \
      die "apply is allowed only from a Kairos recovery boot"
    ;;
  *) usage ;;
esac

resolve_label() {
  label=$1
  path="/dev/disk/by-label/$label"
  [ -e "$path" ] || return 1
  readlink -f "$path"
}

state_part=$(resolve_label COS_STATE) || die "COS_STATE was not found"
old_part=$(resolve_label COS_PERSIST_OLD || true)
persist_part=$(resolve_label COS_PERSISTENT) || \
  die "protected external COS_PERSISTENT was not found"

state_name=$(basename "$state_part")
state_parent=$(lsblk -dnro PKNAME "$state_part")
state_partn=$(lsblk -dnro PARTN "$state_part")
state_partlabel=$(lsblk -dnro PARTLABEL "$state_part")
state_fstype=$(lsblk -dnro FSTYPE "$state_part")

[ -n "$state_parent" ] || die "cannot determine the COS_STATE parent disk"
[ -n "$state_partn" ] || die "cannot determine the COS_STATE partition number"
[ "$state_partlabel" = state ] || \
  die "COS_STATE has unexpected GPT label '$state_partlabel'"
case "$state_fstype" in
  ext2|ext3|ext4) ;;
  *) die "COS_STATE has unsupported filesystem '$state_fstype'" ;;
esac

state_disk="/dev/$state_parent"
persist_parent=$(lsblk -dnro PKNAME "$persist_part")
[ -n "$persist_parent" ] || \
  die "cannot determine the COS_PERSISTENT parent disk"
[ "$persist_parent" != "$state_parent" ] || \
  die "protected COS_PERSISTENT unexpectedly shares boot disk $state_disk"

logical_sector_size=$(cat "/sys/class/block/$state_parent/queue/logical_block_size")
[ "$logical_sector_size" -eq "$sector_bytes" ] || \
  die "boot disk uses $logical_sector_size-byte logical sectors; only 512 is supported"

state_start=$(cat "/sys/class/block/$state_name/start")
state_sectors=$(cat "/sys/class/block/$state_name/size")
state_end=$((state_start + state_sectors - 1))
target_end=$((state_start + target_state_sectors - 1))
disk_sectors=$(cat "/sys/class/block/$state_parent/size")

[ "$target_end" -lt "$disk_sectors" ] || \
  die "boot disk is too small for an ${target_state_mib} MiB COS_STATE"

if [ -n "$old_part" ]; then
  old_name=$(basename "$old_part")
  old_parent=$(lsblk -dnro PKNAME "$old_part")
  old_partn=$(lsblk -dnro PARTN "$old_part")
  old_partlabel=$(lsblk -dnro PARTLABEL "$old_part")
  old_start=$(cat "/sys/class/block/$old_name/start")

  [ "$old_parent" = "$state_parent" ] || \
    die "COS_PERSIST_OLD is not on the COS_STATE boot disk"
  [ "$old_partlabel" = persistent ] || \
    die "COS_PERSIST_OLD has unexpected GPT label '$old_partlabel'"
  [ "$old_start" -eq $((state_end + 1)) ] || \
    die "COS_PERSIST_OLD does not immediately follow COS_STATE"

  for partition_dir in "/sys/class/block/$state_parent"/*; do
    [ -f "$partition_dir/partition" ] || continue
    candidate_name=$(basename "$partition_dir")
    [ "$candidate_name" = "$state_name" ] && continue
    [ "$candidate_name" = "$old_name" ] && continue
    candidate_start=$(cat "$partition_dir/start")
    candidate_size=$(cat "$partition_dir/size")
    candidate_end=$((candidate_start + candidate_size - 1))
    if [ "$candidate_start" -le "$target_end" ] && \
       [ "$candidate_end" -ge "$state_start" ]; then
      die "$candidate_name overlaps the requested COS_STATE extent"
    fi
  done
else
  old_partn=
  [ "$state_sectors" -ge "$target_state_sectors" ] || \
    die "COS_PERSIST_OLD is absent but COS_STATE is smaller than ${target_state_mib} MiB"
fi

mounted_at=$(findmnt -rn -S "$state_part" -o TARGET || true)
if [ -n "$mounted_at" ] && \
   { [ "$mode" = apply ] || [ -f /run/cos/recovery_mode ]; }; then
  die "COS_STATE is mounted at $mounted_at during recovery; refusing offline migration"
fi

echo "mode:                       $mode"
echo "boot disk:                  $state_disk"
echo "COS_STATE:                   $state_part (${state_sectors} sectors)"
echo "COS_PERSIST_OLD:             ${old_part:-absent}"
echo "protected COS_PERSISTENT:    $persist_part (parent /dev/$persist_parent)"
echo "target COS_STATE:            ${target_state_mib} MiB (${target_state_sectors} sectors)"
echo "COS_STATE mounted at:        ${mounted_at:-not mounted}"

if [ -z "$old_part" ]; then
  echo "result: already migrated"
  exit 0
fi

if [ "$state_sectors" -ge "$target_state_sectors" ]; then
  resize_state=false
  echo "planned partition changes:  remove partition $old_partn; COS_STATE already meets target"
else
  resize_state=true
  echo "planned partition changes:  remove partition $old_partn; grow partition $state_partn through sector $target_end"
fi

[ "$mode" = apply ] || {
  echo "result: dry-run only; no writes performed"
  exit 0
}

fsck_ext() {
  set +e
  e2fsck -f -p "$state_part"
  status=$?
  set -e
  case "$status" in
    0|1) ;;
    *) die "e2fsck failed with status $status" ;;
  esac
}

fsck_ext
if [ "$resize_state" = true ]; then
  parted --script --fix "$state_disk" unit s \
    rm "$old_partn" \
    resizepart "$state_partn" "${target_end}s"
else
  parted --script --fix "$state_disk" rm "$old_partn"
fi
partprobe "$state_disk"
udevadm settle

new_state_sectors=$(cat "/sys/class/block/$state_name/size")
if [ "$resize_state" = true ]; then
  [ "$new_state_sectors" -eq "$target_state_sectors" ] || \
    die "kernel reports $new_state_sectors sectors after resize, expected $target_state_sectors"
  fsck_ext
  resize2fs "$state_part"
fi
fsck_ext

[ ! -e /dev/disk/by-label/COS_PERSIST_OLD ] || \
  die "COS_PERSIST_OLD still exists after partition-table update"
[ "$(readlink -f /dev/disk/by-label/COS_PERSISTENT)" = "$persist_part" ] || \
  die "protected COS_PERSISTENT changed unexpectedly"

echo "result: migration completed successfully"
