#!/bin/sh
set -e

# Debugging - Print environment variables
echo "DEBUG: Registry: $REGISTRY"
echo "DEBUG: Username: $REGISTRY_USERNAME"
echo "DEBUG: KO Tags: $KO_TAGS"
echo "DEBUG: DIR: ${DIR:-not set}"

# Login to container registry
echo "Logging into registry $REGISTRY..."
ko login "$REGISTRY" --username "$REGISTRY_USERNAME" --password "$REGISTRY_PASSWORD"

# Publish using ko
echo "Publishing with ko..."
ko publish . --bare --tags="$KO_TAGS" --platform="linux/amd64,linux/arm64"


# Execute additional commands if passed
echo "Starting the container command..."
exec "$@"