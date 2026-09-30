#!/bin/sh
# Runs as root: the podman user's runtime directory and API socket, then
# sshd in the foreground.
set -eu
# Host keys are made here, not in the image, so every new container is a
# server with new keys (task servers:test replaces them on purpose).
ssh-keygen -A >/dev/null
install -d -m 700 -o podman -g podman /run/user/1000 /run/user/1000/podman
su podman -s /bin/sh -c 'XDG_RUNTIME_DIR=/run/user/1000 nohup podman system service --time=0 unix:///run/user/1000/podman/podman.sock >/tmp/podman-service.log 2>&1 &'
# sshd sessions do not get XDG_RUNTIME_DIR from pam_systemd here.
echo 'export XDG_RUNTIME_DIR=/run/user/1000' > /etc/profile.d/xdg.sh
exec /usr/sbin/sshd -D -e
