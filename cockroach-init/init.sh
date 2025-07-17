#!/bin/sh
set -e

# Wait for CockroachDB to be ready
until /cockroach/cockroach sql --insecure --host=roach1 -e "SELECT 1" > /dev/null 2>&1 || \
      /cockroach/cockroach sql --insecure --host=roach2 -e "SELECT 1" > /dev/null 2>&1 || \
      /cockroach/cockroach sql --insecure --host=roach3 -e "SELECT 1" > /dev/null 2>&1; do
  echo "Waiting for CockroachDB..."
  sleep 1
done

# Check if the cluster is already initialized
if /cockroach/cockroach node status --insecure --host=roach1 | grep -q "id"; then
  echo "Cluster already initialized."
else
  echo "Initializing cluster..."
  /cockroach/cockroach init --insecure --host=roach1
fi

# Create the logsdb database if it doesn't exist
/cockroach/cockroach sql --insecure --host=roach1 -e "CREATE DATABASE IF NOT EXISTS logsdb;"
echo "Database 'logsdb' created or already exists."
