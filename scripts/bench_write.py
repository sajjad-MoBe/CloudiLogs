import random
import time
import uuid
import asyncio
import httpx
import time
import statistics
import datetime

PROJECT_ID = "6a5aec3c-8078-4e37-9c7c-139dddd5fa3a"
API_KEY = "2a8a4cdcd2bf84abd13c3e0afe6efc3ae7d80ae3fd468faf1997dc931477587a"
SEARCHABLE_KEYS = ["folan1", "folan2"]

URL = f"http://localhost:8084/api/projects/{PROJECT_ID}/logs"

DURATION = 10
CONCURRENCY = 500
RPS = 1000





def generate_random_payload():
    payload = {
        "name": f"event_{random.randint(1, 5)}",
        "timestamp": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "searchable_keys": {
            x: str(uuid.uuid4()) for x in SEARCHABLE_KEYS
        },
        "full_payload": {
            "ip_address": "127.0.0.1",
            "random_number": random.randint(1, 100)
        }
    }
    return payload



async def benchmark_url(url):
    success = 0
    fail = 0
    latencies = []
    semaphore = asyncio.Semaphore(RPS)

    async def worker(started):
        nonlocal success, fail, latencies
        headers = {
            "Content-Type": "application/json",
            "X-API-KEY": API_KEY
        }
        async with httpx.AsyncClient(headers=headers) as client:
            while time.time() - started < DURATION:
                payload = generate_random_payload()
                async with semaphore:
                    req_start = time.time()
                    try:
                        r = await client.post(url, timeout=10, json=payload)
                        latency = time.time() - req_start
                        if r.status_code == 202: # status created
                            success += 1
                            latencies.append(latency)
                        else:
                            fail += 1
                    except:
                        fail += 1
                await asyncio.sleep(1 / RPS * CONCURRENCY)

    started = time.time()
    tasks = [worker(started) for _ in range(CONCURRENCY)]
    await asyncio.gather(*tasks)
    elapsed = time.time() - started

    total = success + fail
    print(f"\n🔗 URL: {url}")
    print(f"Total Requests: {total}")
    print(f"Successful: {success}")
    print(f"Failed: {fail}")
    if latencies:
        print(f"Avg Latency: {statistics.mean(latencies):.4f}s")
        print(f"Max Latency: {max(latencies):.4f}s")
        print(f"Response Rate: {success / elapsed:.2f} req/sec")


async def main():
    await benchmark_url(URL)


asyncio.run(main())
