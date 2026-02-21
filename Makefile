SHELL := /bin/sh

.PHONY: build test test-race pipeline docker-test docker-race load load-report degradation-report redis-failover-check explain-analyze

build:
	cd backend && go build ./...

test:
	cd backend && go test ./...

test-race:
	cd backend && go test -race ./...

pipeline: build test test-race

docker-test:
	docker compose -f docker/docker-compose.yml run --rm test

docker-race:
	docker compose -f docker/docker-compose.yml run --rm test go test -race ./...

load:
	docker compose -f docker/docker-compose.yml run --rm k6 run loadtest/ws-load.js

load-report:
	powershell -ExecutionPolicy Bypass -File loadtest/run-load-report.ps1

degradation-report:
	powershell -ExecutionPolicy Bypass -File loadtest/run-degradation-report.ps1

redis-failover-check:
	powershell -ExecutionPolicy Bypass -File loadtest/redis-failover-check.ps1

explain-analyze:
	docker exec -i docker-postgres-1 psql -U durak -d durak < backend/scripts/explain_analyze.sql
