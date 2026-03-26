#!/bin/bash
set -euo pipefail

# Configuration from environment
NGINX_URL="${NGINX_URL:-https://nginx}"
BACKEND_HEALTH_PATH="${BACKEND_HEALTH_PATH:-/healthz}"
CHECK_INTERVAL="${CHECK_INTERVAL:-30}"
FAILURE_THRESHOLD="${FAILURE_THRESHOLD:-2}"
GITHUB_REPO="${GITHUB_REPO:-fairyhunter13/ai-cv-evaluator}"

# State
consecutive_failures=0
last_trigger_time=0
cooldown_period=300 # 5 minutes between triggers

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') - $1"
}

trigger_failover() {
  local current_time
  current_time=$(date +%s)
  local time_since_last=$((current_time - last_trigger_time))

  if [ $time_since_last -lt $cooldown_period ]; then
    log "COOLDOWN: Skipping trigger, only ${time_since_last}s since last trigger (cooldown: ${cooldown_period}s)"
    return
  fi

  log "TRIGGERING FAILOVER via GitHub Actions"

  # Trigger GitHub Actions workflow via repository_dispatch
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GH_PAT_TOKEN}" \
    -H "Content-Type: application/json" \
    "https://api.github.com/repos/${GITHUB_REPO}/dispatches" \
    -d '{"event_type":"backend-unhealthy","client_payload":{"source":"health-monitor","timestamp":"'"$(date -Iseconds)"'"}}')

  if [ "$HTTP_CODE" = "204" ]; then
    log "SUCCESS: GitHub Actions triggered (HTTP $HTTP_CODE)"
    last_trigger_time=$current_time
  else
    log "ERROR: Failed to trigger GitHub Actions (HTTP $HTTP_CODE)"
  fi
}

check_disk_space() {
  local usage
  usage=$(df / | tail -1 | awk '{print $5}' | tr -d '%')

  if [ "$usage" -ge 90 ]; then
    log "DISK CRITICAL: ${usage}% used - running emergency cleanup"
    docker image prune -af --filter "until=1h" 2>/dev/null || true
    docker volume prune -f 2>/dev/null || true
    docker builder prune -af --keep-storage=512MB 2>/dev/null || true
    find /var/log -name "*.gz" -mtime +7 -delete 2>/dev/null || true
    local new_usage
    new_usage=$(df / | tail -1 | awk '{print $5}' | tr -d '%')
    log "DISK CLEANUP: ${usage}% -> ${new_usage}%"
  elif [ "$usage" -ge 80 ]; then
    log "DISK WARNING: ${usage}% used - running light cleanup"
    docker image prune -f 2>/dev/null || true
    docker builder prune -af --keep-storage=2GB 2>/dev/null || true
  fi
}

check_nginx_alive() {
  # Check if nginx itself is responding (not the backend)
  # Use a HEAD request to the root - nginx will respond even if backend is down
  # We use --connect-timeout to fail fast if nginx container is completely down
  local status
  status=$(curl -sk -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 "${NGINX_URL}/" 2>/dev/null || echo "000")

  # Any response from nginx (even 502/503) means nginx is alive
  # 000 means connection failed (nginx container down)
  if [ "$status" = "000" ]; then
    return 1 # nginx is down
  fi
  return 0 # nginx is alive
}

check_backend_health() {
  # Check the backend health through nginx
  local status
  status=$(curl -sk -o /dev/null -w "%{http_code}" --max-time 10 "${NGINX_URL}${BACKEND_HEALTH_PATH}" 2>/dev/null || echo "000")

  if [ "$status" = "200" ]; then
    return 0 # healthy
  fi
  echo "$status"
  return 1 # unhealthy
}

log "Starting health monitor"
log "Config: NGINX_URL=$NGINX_URL, Path=$BACKEND_HEALTH_PATH, Interval=${CHECK_INTERVAL}s, Threshold=$FAILURE_THRESHOLD failures"

while true; do
  # Step 0: Check disk space and auto-cleanup if needed
  check_disk_space

  # Step 1: Check if nginx itself is alive
  if ! check_nginx_alive; then
    log "NGINX DOWN: Cannot connect to nginx container - NOT triggering backend failover"
    log "  This is an nginx issue, not a backend issue. Check nginx container logs."
    consecutive_failures=0 # Reset counter - this is not a backend failure
    sleep "$CHECK_INTERVAL"
    continue
  fi

  # Step 2: Nginx is alive, now check backend health through nginx
  if backend_status=$(check_backend_health); then
    if [ $consecutive_failures -gt 0 ]; then
      log "RECOVERED: Backend healthy after $consecutive_failures failures"
    fi
    consecutive_failures=0
  else
    ((consecutive_failures++)) || true
    log "BACKEND FAILURE $consecutive_failures/$FAILURE_THRESHOLD: Health check returned HTTP $backend_status"

    if [ $consecutive_failures -ge $FAILURE_THRESHOLD ]; then
      trigger_failover
    fi
  fi

  sleep "$CHECK_INTERVAL"
done
