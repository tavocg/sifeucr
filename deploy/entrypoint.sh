#!/bin/sh
set -eu

if [ "$#" -gt 0 ]; then
  exec su-exec sifeucr /sifeucr "$@"
fi

: "${SIFEUCR_DB:?Set SIFEUCR_DB to an external database DSN}"
nginx -t

su-exec sifeucr /sifeucr serve --host 127.0.0.1 --port 3080 &
SIFEUCR_PID=$!

nginx -g 'daemon off;' &
NGINX_PID=$!

shutdown() {
  trap '' TERM INT
  # Keep nginx serving while Go withdraws readiness and drains requests.
  kill -TERM "$SIFEUCR_PID" 2>/dev/null || true
  wait "$SIFEUCR_PID" 2>/dev/null || true
  kill -QUIT "$NGINX_PID" 2>/dev/null || true
  wait "$NGINX_PID" 2>/dev/null || true
}

trap 'shutdown; exit 0' TERM INT

# Polling also catches children that exited before we began waiting.
while kill -0 "$SIFEUCR_PID" 2>/dev/null &&
  kill -0 "$NGINX_PID" 2>/dev/null; do
  sleep 1
done

echo "Go or nginx exited unexpectedly; stopping container." >&2

shutdown
exit 1
