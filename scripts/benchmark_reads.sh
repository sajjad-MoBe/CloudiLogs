root@server18815:~# cat bench.sh 
#!/bin/bash

# Usage: ./scripts/benchmark_reads.sh <USERNAME> <PASSWORD> <PROJECT_ID> <EVENT_NAME>

set -e

USERNAME=$1
PASSWORD=$2
PROJECT_ID=$3
EVENT_NAME=$4

if [ -z "$USERNAME" ] || [ -z "$PASSWORD" ] || [ -z "$PROJECT_ID" ] || [ -z "$EVENT_NAME" ]; then
    echo "Usage: $0 <USERNAME> <PASSWORD> <PROJECT_ID> <EVENT_NAME>"
    exit 1
fi

# --- 1. Log in and save the session cookie ---
COOKIE_JAR=$(mktemp)
LOGIN_URL="http://localhost:8083/api/auth/login"
LOGS_URL="http://localhost:8083/api/projects/${PROJECT_ID}/logs?event_name=${EVENT_NAME}"

JSON_PAYLOAD=$(cat <<EOF
{
    "username": "$USERNAME",
    "password": "$PASSWORD"
}
EOF
)

echo "Attempting to log in as '$USERNAME'..."
LOGIN_RESPONSE=$(curl -s -c "$COOKIE_JAR" -X POST "$LOGIN_URL" -H "Content-Type: application/json" -d "$JSON_PAYLOAD")
echo $LOGIN_RESPONSE
if ! echo "$LOGIN_RESPONSE" | grep -q "success"; then
    echo "Login failed. Please check your username and password."
    rm "$COOKIE_JAR"
    exit 1
fi
echo "Login successful. Cookie saved to $COOKIE_JAR"


DURATION=3
CONCURRENT_REQUESTS=10

echo "Starting benchmark for $DURATION seconds with $CONCURRENT_REQUESTS concurrent requests..."
echo "Target URL: $LOGS_URL"
echo "------------------------------------"

SUCCESSFUL_REQUESTS=0
(
    while true; do
        for (( i=1; i<=CONCURRENT_REQUESTS; i++ )); do
            curl -s -b "$COOKIE_JAR" "$LOGS_URL" > /dev/null && echo "success" &
        done
        wait
    done
) | pv -l -t -i 0.5 -r -F "Requests: %c | Rate: %r req/s" > >(
    while read -r line; do
        ((SUCCESSFUL_REQUESTS++))
    done
) &

BENCHMARK_PID=$!

sleep $DURATION

rm "$COOKIE_JAR"
echo ""
echo "------------------------------------"
echo "Benchmark finished."

FINAL_COUNT=$SUCCESSFUL_REQUESTS
REQUESTS_PER_SECOND=$(echo "scale=2; $FINAL_COUNT / $DURATION" | bc)

echo "Total successful requests in $DURATION seconds: $FINAL_COUNT"
echo "Approximate reads per second: $REQUESTS_PER_SECOND"