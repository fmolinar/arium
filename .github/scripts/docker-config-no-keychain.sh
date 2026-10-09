#!/usr/bin/env bash
# Points this job's docker CLI at a config whose credential helper never
# touches the macOS Keychain. The runner is a launchd service with no login
# session, so the Keychain can't be opened and every pull fails with "error
# getting credentials", even for public images. Dropping credsStore isn't
# enough: on macOS the CLI then falls back to osxkeychain whenever
# docker-credential-osxkeychain is on PATH, which Docker Desktop puts there.
# So the config names "anonymous", a helper that never has credentials.
# Nothing here logs in to a registry. ~/.docker itself is left alone.
set -euo pipefail

[ "${RUNNER_OS:-}" = macOS ] || exit 0

src="$HOME/.docker"
dir="$RUNNER_TEMP/docker-config"
mkdir -p "$dir"

# Docker Desktop's compose/buildx plugins live in ~/.docker/cli-plugins or the
# app bundle; keep both reachable.
[ -d "$src/cli-plugins" ] && ln -sfn "$src/cli-plugins" "$dir/cli-plugins"
cat > "$dir/config.json" <<JSON
{
  "credsStore": "anonymous",
  "cliPluginsExtraDirs": ["/Applications/Docker.app/Contents/Resources/cli-plugins"]
}
JSON

# The docker-credential-helpers protocol: "get" answering with this exact
# not-found message makes the CLI pull anonymously.
mkdir -p "$dir/bin"
cat > "$dir/bin/docker-credential-anonymous" <<'SH'
#!/bin/sh
cat >/dev/null
case "$1" in
  get) echo "credentials not found in native keychain"; exit 1 ;;
  list) echo "{}" ;;
esac
SH
chmod +x "$dir/bin/docker-credential-anonymous"
echo "$dir/bin" >> "$GITHUB_PATH"

# Without ~/.docker/contexts the CLI falls back to /var/run/docker.sock, which
# Docker Desktop only creates when that option is enabled, so name its socket.
if [ -S "$src/run/docker.sock" ]; then
  echo "DOCKER_HOST=unix://$src/run/docker.sock" >> "$GITHUB_ENV"
fi
echo "DOCKER_CONFIG=$dir" >> "$GITHUB_ENV"
