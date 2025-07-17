#!/bin/sh
set -e

sleep 15
set +e
/cockroach/cockroach init --insecure --host=roach1
set -e


# Create the logsdb database if it doesn't exist
/cockroach/cockroach sql --insecure --host=roach1 -e "CREATE DATABASE IF NOT EXISTS logsdb;"
echo "Database 'logsdb' created or already exists."
