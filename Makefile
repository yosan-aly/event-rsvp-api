TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:5433/event_rsvp?sslmode=disable

.PHONY: run docs test test-integration vet build docker-build

run:
	go run ./cmd

docs:
	sh scripts/open-api.sh

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -count=1 ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags="-s -w" -o bin/event-rsvp-api ./cmd

docker-build:
	docker build -t event-rsvp-api:local .
