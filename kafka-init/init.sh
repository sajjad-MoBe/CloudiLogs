#!/bin/sh
set -e

sleep 5

# Create the log-events topic if it doesn't exist
if kafka-topics --bootstrap-server kafka1:9092 --list | grep -q "log-events"; then
  echo "Topic 'log-events' already exists."
else
  echo "Creating topic 'log-events'..."
  kafka-topics --bootstrap-server kafka1:9092 --create --topic log-events --partitions 1 --replication-factor 3
fi
