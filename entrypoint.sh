#!/bin/sh
set -e

# Replace RabbitMQ config with environment variables
# We use a temp file to avoid race conditions and ensure clean replacement
cp config.yaml config.yaml.tmp

# Replace Host
if [ ! -z "$RABBITMQ_HOST" ]; then
  sed -i "s/host: localhost/host: $RABBITMQ_HOST/g" config.yaml.tmp
fi

# Replace Password
if [ ! -z "$RABBITMQ_DEFAULT_PASS" ]; then
  # Escape special characters in password if needed, but for now assuming simple
  sed -i "s/password: secret/password: $RABBITMQ_DEFAULT_PASS/g" config.yaml.tmp
fi

# Replace Clerk Secret (if present in config, though code loads it from env usually)
# The code loads CLERK_SECRET_KEY from env, so we might not need to replace it in yaml

mv config.yaml.tmp config.yaml

echo "🔧 Configuration updated:"
grep "host:" config.yaml

echo "🚀 Starting service..."
exec ./service
