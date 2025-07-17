#!/bin/sh
set -e

sleep(15)
/cockroach/cockroach init --insecure --host=roach1


# Create the logsdb database if it doesn't exist
/cockroach/cockroach sql --insecure --host=roach1 -e "CREATE DATABASE IF NOT EXISTS logsdb;"
echo "Database 'logsdb' created or already exists."
