.PHONY: build run test test-unit test-integration lint clean up down logs

build:
	go build -o bin/api ./cmd/api/

run: build
	./bin/api

test: test-unit test-integration

test-unit:
	go test ./internal/... -race -v

test-integration:
	go test ./tests/ -race -v -count=1

lint:
	go vet ./...

clean:
	rm -f bin/api
	docker compose down -v

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

smoke:
	@echo "Creating person..."
	@curl -s -w "\n%{http_code}\n" -X POST http://localhost:9999/pessoas \
		-H "Content-Type: application/json" \
		-d '{"apelido":"smoke","nome":"Smoke Test","nascimento":"2000-01-01","stack":["Go"]}'
	@echo "\nCount:"
	@curl -s http://localhost:9999/contagem-pessoas
	@echo ""
