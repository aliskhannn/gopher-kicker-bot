.PHONY: build run dev clean docker-up docker-down

build:
	go build -o bin/bot ./cmd/bot

run: build
	./bin/bot

dev:
	docker-compose up --build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v

clean:
	rm -rf bin/ coverage.out

test:
	go test ./...
