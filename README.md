# Log Analysis System

This project is a scalable and fault-tolerant log analysis system designed to ingest, store, and analyze a high volume of log data. It uses a microservices architecture and leverages modern distributed systems technologies like Kafka, Cassandra, ClickHouse, and CockroachDB.

## Contributors

*   Sajjad Mohammadbeigi
*   Mohammad Mohammadbeigi
*   Nazanin Yousefi

## Architecture Overview

The system is composed of several services that work together to provide a complete logging pipeline.

*   **`frontend`**: A simple web interface for user authentication, project management, and log visualization.
*   **`backend-api`**: The main entry point for the system. It handles user requests, authentication (via CockroachDB), and ingests logs by producing them to Kafka.
*   **`cockroachdb`**: A 3-node, distributed SQL database used to store user and project metadata. It provides high availability for critical user data.
*   **`kafka`**: A 3-node, distributed messaging system that acts as a durable buffer for incoming logs. This allows the system to handle high write throughput without losing data.
*   **`zookeeper`**: A 3-node ensemble required for coordinating the Kafka cluster.
*   **`log-processor`**: A service that consumes logs from Kafka and performs a dual-write to two different databases for two different purposes:
    *   **`cassandra`**: A 3-node, distributed NoSQL database used for long-term archival of the full, raw log payloads. Optimized for high write throughput.
    *   **`clickhouse`**: A single-node columnar database optimized for fast analytical queries. It stores indexed, searchable fields from the logs to power the search and aggregation features in the UI.

### Log Ingestion Flow

1.  A client sends a log to the `backend-api`.
2.  The `backend-api` authenticates the request against **CockroachDB**.
3.  The log is published to a topic in the **Kafka** cluster.
4.  The `log-processor` consumes the log from Kafka.
5.  The full log payload is stored in **Cassandra**.
6.  Indexed metadata and searchable keys are stored in **ClickHouse**.

### Log Retrieval Flow

1.  The user's browser requests aggregated log data from the `backend-api`.
2.  The `backend-api` runs a fast analytical query on **ClickHouse** to get counts, last seen times, etc.
3.  When a user requests to see the full details of a specific log, the `backend-api` retrieves the full payload from **Cassandra**.

## How to Run the System

### Prerequisites

*   Docker
*   Docker Compose
*   `pv` (Pipe Viewer) for the benchmark script:
    *   **macOS**: `brew install pv`
    *   **Debian/Ubuntu**: `sudo apt-get install pv`

### 1. Start the System

From the root of the project, run:

```bash
docker-compose up --build -d
```

This will build the necessary Docker images and start all the services in the background. It may take a few minutes for all services to become healthy, especially on the first run.

### 2. Create a User

You can create a new user with the provided script.

```bash
./scripts/create_user.sh <username> <password> "Your Name" <email@example.com>
```

### 3. Use the Web Interface

Navigate to `http://localhost:8084` in your browser. You can log in with the user you just created, create a new project, get an API key, and start sending logs.

### 4. Send Test Logs

You can use the provided scripts to send test logs to your project.

*   **Send a single test log:**
    ```bash
    ./scripts/test_log_ingestion.sh <YOUR_PROJECT_ID> <YOUR_API_KEY>
    ```

*   **Run a write load test (4 million logs):**
    ```bash
    ./scripts/load_test.sh <YOUR_PROJECT_ID> <YOUR_API_KEY>
    ```


*   **Run a write benchmark test:**
    ##### edit scripts/bench_write.py (lines 10:13)
    ```
    python3 scripts/bench_write.py 
    ```

*   **Run a read benchmark test:**
    ##### edit scripts/bench_read.py (lines 6:10)
    ```
    python3 scripts/bench_read.py 
    ```
