#!/bin/bash

# A script to send a large volume of sample logs to the log ingestion API.
#
# Usage: ./scripts/load_test.sh <PROJECT_ID> <API_KEY>

set -e

PROJECT_ID=$1
API_KEY=$2

if [ -z "$PROJECT_ID" ] || [ -z "$API_KEY" ]; then
    echo "Usage: $0 <PROJECT_ID> <API_KEY>"
    echo "Please provide the Project ID and the API Key."
    exit 1
fi

TOTAL_LOGS=4000000
EVENT_NAMES=("login_success" "page_view")

echo "Starting load test to send $TOTAL_LOGS logs..."
echo "Project ID: $PROJECT_ID"
echo "API Key: [REDACTED]"
echo "------------------------------------"

for (( i=1; i<=TOTAL_LOGS; i++ )); do
    EVENT_NAME=${EVENT_NAMES[$((i % 2))]}
    
    TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Generate random values for keys
    FOLAN1_VAL="user-$((1 + RANDOM % 1000))"
    FOLAN2_VAL=$(uuidgen)
    RANDOM_KEY_1="val-$((1 + RANDOM % 100))"
    RANDOM_KEY_2="status_code_$((200 + RANDOM % 4))"

    JSON_PAYLOAD='{
      "name": "'"$EVENT_NAME"'",
      "timestamp": "'"$TIMESTAMP"'",
      "searchable_keys": {
        "folan1": "'"$FOLAN1_VAL"'",
        "folan2": "'"$FOLAN2_VAL"'"
      },
      "full_payload": {
        "random_key_1": "'"$RANDOM_KEY_1"'",
        "random_key_2": "'"$RANDOM_KEY_2"'"
      }
    }'

    curl -s -X POST "http://localhost:8083/api/projects/${PROJECT_ID}/logs" \
    -H "Content-Type: application/json" \
    -H "X-API-KEY: ${API_KEY}" \
    -d "$JSON_PAYLOAD" &

    # manage concurrency, wait for all background jobs to finish every 100 requests.
    if (( i % 100 == 0 )); then
        wait
        echo "Sent $i / $TOTAL_LOGS logs..."
    fi
done

wait

echo "------------------------------------"
echo "Load test complete. All $TOTAL_LOGS log ingestion requests sent."