#!/bin/sh
# Times docker exec + pg_dump of a throwaway ~2MB database.
# Isolates the local (non-SSH) dump path from the API, the worker and storage.
# Removes the container on exit. Fails if pg_dump is still running after 60s.
set -eu

name=backapeando-dump-smoke
out="${TMPDIR:-/tmp}/backapeando-dump-smoke.dump"
password=smoke

cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	rm -f "$out"
}
trap cleanup EXIT INT TERM

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required" >&2
	exit 1
fi

echo "starting throwaway postgres ($name)"
docker rm -f "$name" >/dev/null 2>&1 || true
docker run -d --name "$name" \
	-e POSTGRES_PASSWORD="$password" \
	-e POSTGRES_USER=smoke \
	-e POSTGRES_DB=smoke \
	postgres:16-alpine >/dev/null

echo "waiting until database smoke accepts a query"
ready=0
i=0
while [ "$i" -lt 40 ]; do
	if docker exec -e PGPASSWORD="$password" "$name" \
		psql -v ON_ERROR_STOP=1 -U smoke -d smoke -c "SELECT 1" >/dev/null 2>&1; then
		ready=1
		break
	fi
	i=$((i + 1))
	sleep 1
done
if [ "$ready" -ne 1 ]; then
	echo "postgres did not become ready in 40s" >&2
	docker logs "$name" >&2 || true
	exit 1
fi

echo "loading about 2MB (10000 rows x 200 bytes)"
docker exec -e PGPASSWORD="$password" "$name" \
	psql -v ON_ERROR_STOP=1 -U smoke -d smoke -c \
	"CREATE TABLE smoke_payload (id int, payload text); INSERT INTO smoke_payload SELECT g, repeat('x', 200) FROM generate_series(1, 10000) AS g;" \
	>/dev/null

echo "running pg_dump -Fc via docker exec (limit 60s)"
start=$(date +%s)
docker exec -e PGPASSWORD="$password" "$name" \
	pg_dump -U smoke -Fc smoke >"$out" &
pid=$!
i=0
while [ "$i" -lt 60 ]; do
	if ! kill -0 "$pid" 2>/dev/null; then
		break
	fi
	i=$((i + 1))
	sleep 1
done
if kill -0 "$pid" 2>/dev/null; then
	kill "$pid" 2>/dev/null || true
	wait "$pid" 2>/dev/null || true
	echo "pg_dump still running after 60s. The hang is in docker exec/pg_dump, before storage." >&2
	exit 1
fi
set +e
wait "$pid"
status=$?
set -e
if [ "$status" -ne 0 ]; then
	echo "pg_dump exited $status" >&2
	exit "$status"
fi
end=$(date +%s)
bytes=$(wc -c <"$out" | tr -d ' ')
elapsed=$((end - start))

echo "dump finished in ${elapsed}s, ${bytes} bytes"
if [ "$bytes" -lt 1000 ]; then
	echo "dump is unexpectedly small (${bytes} bytes)" >&2
	exit 1
fi
