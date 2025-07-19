import asyncio
import httpx
import time
import statistics

USERNAME = "sajjad"
PASSWORD = "sajjad"
LOGIN_URL = "http://localhost:8084/api/auth/login"
PROJECT_ID = "6a5aec3c-8078-4e37-9c7c-139dddd5fa3a"
SEARCH_QUERY = "event_name=&start_time=2025-07-14T21%3A09&end_time=2025-07-20T21%3A09&search_keys=folan1%3Auser-3630"


URLS = [
    f"http://localhost:8084/api/projects/{PROJECT_ID}/logs/aggregated",
    f"http://localhost:8084/api/projects/{PROJECT_ID}/logs?{SEARCH_QUERY}"
]

DURATION = 10
CONCURRENCY = 100
RPS = 700


async def login():
    async with httpx.AsyncClient() as client:
        r = await client.post(LOGIN_URL, json={
            "username": USERNAME,
            "password": PASSWORD
        })
        r.raise_for_status()
        return client.cookies


async def benchmark_url(url, cookies):
    success = 0
    fail = 0
    latencies = []
    semaphore = asyncio.Semaphore(RPS)

    async def worker(started):
        nonlocal success, fail, latencies
        async with httpx.AsyncClient(cookies=cookies) as client:
            while time.time() - started < DURATION:
                async with semaphore:
                    req_start = time.time()
                    try:
                        r = await client.get(url, timeout=10)
                        latency = time.time() - req_start
                        if r.status_code == 200:
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
    cookies = await login()
    for url in URLS:
        await benchmark_url(url, cookies)


asyncio.run(main())
