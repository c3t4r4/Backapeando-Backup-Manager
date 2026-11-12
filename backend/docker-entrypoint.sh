#!/bin/sh
# Starts as root only to put appuser in the group that owns the Docker
# socket, then drops privileges. SSH-only deploys that do not mount the
# socket skip the group change.
set -eu

if [ -S /var/run/docker.sock ]; then
  gid=$(stat -c '%g' /var/run/docker.sock)
  groupname=$(awk -F: -v gid="$gid" '$3 == gid { print $1; exit }' /etc/group)
  if [ -z "$groupname" ]; then
    groupname=dockersock
    addgroup -g "$gid" -S "$groupname"
  fi
  addgroup appuser "$groupname"
fi

exec su-exec appuser "$@"
