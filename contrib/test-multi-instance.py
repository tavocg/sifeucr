#!/usr/bin/env python3
"""Disposable Docker smoke test; requires Python 3 and a built sifeucr image."""
import concurrent.futures
import json
import os
import subprocess
import time
import urllib.error
import urllib.request
import uuid

IMAGE = os.environ.get("SIFEUCR_TEST_IMAGE", "sifeucr:multi-instance")
PREFIX = "sifeucr-smoke-" + uuid.uuid4().hex[:8]
containers = []


def docker(*args, check=True):
    return subprocess.run(["docker", *args], check=check, text=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout.strip()


def start(name, *args):
    name = PREFIX + "-" + name
    docker("run", "-d", "--name", name, "--network", PREFIX, *args)
    containers.append(name)
    return name


def wait(check, timeout=120):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        try:
            if check():
                return
        except (OSError, subprocess.CalledProcessError, AssertionError):
            pass
        time.sleep(0.2)
    raise AssertionError("timed out waiting for service")


def request(url, path, body=None, token=None, method=None, expected=204):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url + path, data=data, headers=headers, method=method)
    try:
        response = urllib.request.urlopen(req, timeout=5)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        raw = response.read()
        assert response.status == expected, (path, response.status, expected, raw)
        return json.loads(raw) if raw else None


def rejected_start(name, dsn, message):
    container = start(name, "-e", "SIFEUCR_DB=" + dsn,
                      "-e", "SIFEUCR_AUTH_SECRET=" + "a" * 32,
                      "-e", "SIFEUCR_AUTH_SMTP_ADDRESS=smtp.example.com:465",
                      IMAGE, "serve", "--auto-migrate=false")
    wait(lambda: docker("inspect", "-f", "{{.State.Running}}", container) == "false", 15)
    assert docker("inspect", "-f", "{{.State.ExitCode}}", container) != "0"
    logs = subprocess.run(["docker", "logs", container], capture_output=True, text=True)
    assert message in logs.stdout + logs.stderr, logs.stdout + logs.stderr


def exercise(backend):
    if backend == "postgres":
        db = start(backend, "--network-alias", backend, "-e", "POSTGRES_PASSWORD=test",
                   "-e", "POSTGRES_DB=sifeucr", "postgres:18-alpine")
        wait(lambda: "accepting connections" in docker("exec", db, "pg_isready", "-h", "127.0.0.1", "-U", "postgres"))
        admin = "postgres://postgres:test@postgres/sifeucr?sslmode=disable"
        runtime = "postgres://runtime:test@postgres/sifeucr?sslmode=disable"
        sql = ["exec", db, "psql", "-U", "postgres", "-d", "sifeucr", "-v", "ON_ERROR_STOP=1", "-c"]
        grants = "CREATE USER runtime PASSWORD 'test'; GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO runtime; GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO runtime;"
    else:
        db = start(backend, "--network-alias", backend, "-e", "MYSQL_ROOT_PASSWORD=test",
                   "-e", "MYSQL_DATABASE=sifeucr", "mysql:8.4")
        sql = ["exec", db, "mysql", "--protocol=TCP", "-h127.0.0.1", "-uroot", "-ptest", "sifeucr", "-e"]
        wait(lambda: "1" in docker(*sql, "SELECT 1"))
        admin = "root:test@tcp(mysql:3306)/sifeucr"
        runtime = "runtime:test@tcp(mysql:3306)/sifeucr"
        grants = "CREATE USER 'runtime'@'%' IDENTIFIED BY 'test'; GRANT SELECT, INSERT, UPDATE, DELETE ON sifeucr.* TO 'runtime'@'%';"
    rejected_start(backend + "-missing", admin, "schema version")
    for _ in range(2):  # Re-running the migration job must leave a completed schema intact.
        docker("run", "--rm", "--network", PREFIX, "-e", "SIFEUCR_DB=" + admin, IMAGE, "migrate")
    for delta in (-1, 1):
        docker(*sql, "UPDATE schema_migrations SET version = version + " + str(delta))
        rejected_start(backend + "-version-" + str(delta), admin, "schema version")
        docker(*sql, "UPDATE schema_migrations SET version = version - " + str(delta))
    if backend == "mysql":
        docker(*sql, "INSERT INTO schema_migration_pending SELECT id, version FROM schema_migrations")
        rejected_start("mysql-pending", admin, "incomplete")
        docker(*sql, "DELETE FROM schema_migration_pending")
    docker(*sql, grants)
    common = ["--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
              "-p", "127.0.0.1::3080", "-e", "SIFEUCR_DB=" + runtime,
              "-e", "SIFEUCR_AUTH_SECRET=" + "a" * 32, "-e", "SIFEUCR_AUTH_SMTP_ADDRESS=smtp.example.com:465",
              IMAGE, "serve", "--dev", "--logfmt", "json", "--loglvl", "debug", "--auto-migrate=false",
              "--db-max-connections", "2", "--root", "/api", "--drain-delay", "2s"]
    a = start(backend + "-a", *common)
    b = start(backend + "-b", *common)
    urls = ["http://" + docker("port", c, "3080/tcp") for c in [a, b]]
    for url in urls:
        wait(lambda: request(url, "/readyz") is None)
        request(url, "/livez")
    ua, ub = urls
    email = "replicas@example.com"
    tokens = request(ua, "/api/auth/login", {"email": email}, expected=200)
    access = tokens["access_token"]
    assert request(ua, "/api/account", token=access, expected=200)["email"] == email

    def refresh(url):
        # Record both outcomes without assuming which replica wins.
        try:
            return request(url, "/api/auth/refresh", {"refresh_token": tokens["refresh_token"]}, expected=200)
        except AssertionError as err:
            assert err.args[0][1] == 401, err
            return None
    with concurrent.futures.ThreadPoolExecutor(2) as pool:
        results = list(pool.map(refresh, urls))
    assert sum(result is not None for result in results) == 1
    fresh = next(result for result in results if result)
    request(ua, "/api/account/sessions", token=fresh["access_token"], method="DELETE")
    request(ub, "/api/account", token=fresh["access_token"], expected=401)
    request(ub, "/api/auth/refresh", {"refresh_token": fresh["refresh_token"]}, expected=401)
    for i in range(5):
        request(urls[i % 2], "/api/auth/login", {"email": "budget@example.com"}, expected=200)
    request(ub, "/api/auth/login", {"email": "budget@example.com"}, expected=429)

    docker("kill", "--signal", "TERM", a)
    # Development skips the readiness drain delay, so the listener may already
    # be closed when SIGTERM returns. Check the clean exit and surviving replica.
    wait(lambda: docker("inspect", "-f", "{{.State.Running}}", a) == "false")
    assert docker("inspect", "-f", "{{.State.ExitCode}}", a) == "0"
    request(ub, "/readyz")
    # A dependency outage affects readiness, never liveness. Restore and recover.
    docker("pause", db)
    try:
        request(ub, "/readyz", expected=503)
        request(ub, "/livez")
    finally:
        docker("unpause", db)
    wait(lambda: request(ub, "/readyz") is None)
    docker("stop", "-t", "35", b)
    assert docker("inspect", "-f", "{{.State.ExitCode}}", b) == "0"
    print(backend + ": cross-replica login, quotas, profile, refresh, revocation, health and SIGTERM passed", flush=True)


docker("network", "create", PREFIX)
try:
    for backend in ("postgres", "mysql"):
        exercise(backend)
except Exception:
    for container in containers:
        if container.endswith(("-a", "-b")):
            print(docker("logs", container, check=False))
    raise
finally:
    for container in reversed(containers):
        docker("rm", "-f", container, check=False)
    docker("network", "rm", PREFIX, check=False)
