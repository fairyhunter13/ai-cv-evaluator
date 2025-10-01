#!/bin/sh

case "$1" in
  "server")
    exec /app-server
    ;;
  "worker")
    exec /app-worker
    ;;
  *)
    echo "Usage: $0 {server|worker}"
    exit 1
    ;;
esac
