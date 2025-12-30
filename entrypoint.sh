#!/bin/sh
set -e

# Replace RabbitMQ config with environment variables
# We use a temp file to avoid race conditions and ensure clean replacement
cp config.yaml config.yaml.tmp

# Replace Connection Details
[ ! -z "$RABBITMQ_HOST" ] && sed -i "s/host: .*/host: $RABBITMQ_HOST/g" config.yaml.tmp
[ ! -z "$RABBITMQ_PORT" ] && sed -i "s/port: .*/port: $RABBITMQ_PORT/g" config.yaml.tmp
[ ! -z "$RABBITMQ_VHOST" ] && sed -i "s/vhost: .*/vhost: $RABBITMQ_VHOST/g" config.yaml.tmp

# Replace Credentials
[ ! -z "$RABBITMQ_DEFAULT_USER" ] && sed -i "s/username: .*/username: $RABBITMQ_DEFAULT_USER/g" config.yaml.tmp
[ ! -z "$RABBITMQ_DEFAULT_PASS" ] && sed -i "s/password: .*/password: $RABBITMQ_DEFAULT_PASS/g" config.yaml.tmp

mv config.yaml.tmp config.yaml

echo "🔧 Configuration updated for RabbitMQ at $RABBITMQ_HOST"

echo "🚀 Starting service..."
exec ./service
