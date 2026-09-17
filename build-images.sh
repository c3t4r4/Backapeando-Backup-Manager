#!/usr/bin/env bash
set -euo pipefail

# Error handler: print clear failure message with phase context
trap 'echo "ERROR: build-images.sh failed (line $LINENO). prod-version was NOT updated." >&2; exit 1' ERR

# Navigate to the script directory (repo root)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Read the current version from prod-version
if [[ ! -f prod-version ]]; then
  echo "Error: prod-version file not found at $SCRIPT_DIR"
  exit 1
fi

CURRENT_VERSION=$(cat prod-version)

# Validate version format: vMAJOR.MINOR.PATCH
if [[ ! $CURRENT_VERSION =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "Error: prod-version format invalid. Expected vMAJOR.MINOR.PATCH, got: $CURRENT_VERSION"
  exit 1
fi

MAJOR="${BASH_REMATCH[1]}"
MINOR="${BASH_REMATCH[2]}"
PATCH="${BASH_REMATCH[3]}"

# Calculate new version by incrementing patch
NEW_PATCH=$((PATCH + 1))
NEW_VERSION="v${MAJOR}.${MINOR}.${NEW_PATCH}"

echo "Current version: $CURRENT_VERSION"
echo "Building and pushing: $NEW_VERSION"
echo ""

# Define build components (parallel arrays for compatibility with bash 3.2)
# names[i], dockerfiles[i], contexts[i]
declare -a names=("backend" "worker" "frontend")
declare -a dockerfiles=("backend/cmd/api/Dockerfile" "backend/cmd/worker/Dockerfile" "frontend/Dockerfile")
declare -a contexts=("backend" "backend" "frontend")

# Phase 1: Build all images
echo "=== Phase 1: Building Docker images ==="
for i in "${!names[@]}"; do
  name="${names[$i]}"
  dockerfile="${dockerfiles[$i]}"
  context="${contexts[$i]}"

  echo "Building ${name}..."
  docker build \
    -f "$dockerfile" \
    -t "c3t4r4/backapeando:${name}-${NEW_VERSION}" \
    -t "c3t4r4/backapeando:${name}-latest" \
    "$context"
  echo "✓ ${name} built successfully"
done

echo ""
echo "=== Phase 2: Pushing Docker images ==="

# Phase 2: Push all images (only if all builds succeeded)
for i in "${!names[@]}"; do
  name="${names[$i]}"

  echo "Pushing ${name}-${NEW_VERSION} and ${name}-latest..."
  docker push "c3t4r4/backapeando:${name}-${NEW_VERSION}"
  docker push "c3t4r4/backapeando:${name}-latest"
  echo "✓ ${name} pushed successfully"
done

echo ""
echo "=== Phase 3: Updating version and committing ==="

# Phase 3: Update prod-version and commit (only if all builds and pushes succeeded)
# Write atomically to avoid truncated file on kill/crash
PROD_VERSION_TMP="${SCRIPT_DIR}/prod-version.tmp"
printf '%s\n' "$NEW_VERSION" > "$PROD_VERSION_TMP"
mv "$PROD_VERSION_TMP" prod-version

git add prod-version
git commit -m "chore(docker): bump version to ${NEW_VERSION}

Build and push backend (API), worker, and frontend images:
- c3t4r4/backapeando:backend-${NEW_VERSION}
- c3t4r4/backapeando:worker-${NEW_VERSION}
- c3t4r4/backapeando:frontend-${NEW_VERSION}

Plus latest tags for each component."

echo ""
echo "=== Success ==="
echo "Published images:"
echo "  - c3t4r4/backapeando:backend-${NEW_VERSION}"
echo "  - c3t4r4/backapeando:backend-latest"
echo "  - c3t4r4/backapeando:worker-${NEW_VERSION}"
echo "  - c3t4r4/backapeando:worker-latest"
echo "  - c3t4r4/backapeando:frontend-${NEW_VERSION}"
echo "  - c3t4r4/backapeando:frontend-latest"
echo ""
echo "Version bumped to: $NEW_VERSION"
