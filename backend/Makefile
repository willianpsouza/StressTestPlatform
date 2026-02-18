.PHONY: build run test clean migrate seed

build:
	go build -o tmp/api ./cmd/api
	go build -o tmp/migrate ./cmd/migrate
	go build -o tmp/seed ./cmd/seed

run:
	go run ./cmd/api

test:
	go test ./...

clean:
	rm -rf tmp/

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

seed:
	go run ./cmd/seed
