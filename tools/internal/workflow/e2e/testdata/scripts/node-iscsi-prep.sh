set -eu
echo 'k2-tools: preparing iSCSI initiator'
new_iqn="$(iscsi-iname)"
printf 'InitiatorName=%s\n' "$new_iqn" | sudo tee /etc/iscsi/initiatorname.iscsi >/dev/null
sudo systemctl enable --now iscsid
sudo systemctl restart iscsid
