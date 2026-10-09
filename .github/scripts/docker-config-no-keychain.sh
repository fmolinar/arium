#!/usr/bin/env bash
# Points this job's docker CLI at a config without Docker Desktop's credential
# helper. The runner is a launchd service with no login session, so the
# helper can't open the macOS Keychain and every pull fails with "error
# getting credentials", even for public images. Nothing here logs in to a
# registry, so the helper isn't needed. ~/.docker itself is left alone.
set -euo pipefail

[ "${RUNNER_OS:-}" = macOS ] || exit 0

src="$HOME/.docker"
dir="$RUNNER_TEMP/docker-config"
mkdir -p "$dir"

# Docker Desktop's compose/buildx plugins live in ~/.docker/cli-plugins or the
# app bundle; keep both reachable.
[ -d "$src/cli-plugins" ] && ln -sfn "$src/cli-plugins" "$dir/cli-plugins"
cat > "$dir/config.json" <<JSON
{"cliPluginsExtraDirs": ["/Applications/Docker.app/Contents/Resources/cli-plugins"]}
JSON

# Without ~/.docker/contexts the CLI falls back to /var/run/docker.sock, which
# Docker Desktop only creates when that option is enabled, so name its socket.
if [ -S "$src/run/docker.sock" ]; then
  echo "DOCKER_HOST=unix://$src/run/docker.sock" >> "$GITHUB_ENV"
fi
echo "DOCKER_CONFIG=$dir" >> "$GITHUB_ENV"
