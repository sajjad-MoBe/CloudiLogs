#!/bin/bash

# A script to create a new user in the log system.
# Usage: ./scripts/create_user.sh <username> <password> [email] [fullname]

set -e

USERNAME=${1:-"sajjad"}
PASSWORD=${2:-"sajjad"}
EMAIL=${3:-"sajjad@beigi.com"}
FULLNAME=${4:-"aghaye sajjad"}

if ! docker-compose ps backend-api | grep -q "Up"; then
    echo "Error: The 'backend-api' service is not running. Please start the services with 'docker-compose up -d'."
    exit 1
fi

echo "Creating user with the following details:"
echo "Username: $USERNAME"
echo "Password: [REDACTED]"
echo "Email: $EMAIL"
echo "Full Name: $FULLNAME"
echo "------------------------------------"

docker-compose exec backend-api /app/user-creator \
    -username "$USERNAME" \
    -password "$PASSWORD" \
    -email "$EMAIL" \
    -fullname "$FULLNAME"

echo "------------------------------------"
echo "User creation command executed successfully."
echo "You can now log in with these credentials at http://localhost:8084"
