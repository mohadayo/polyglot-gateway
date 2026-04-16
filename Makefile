.PHONY: up down test test-python test-go test-ts lint lint-python lint-go lint-ts build clean

# Docker Compose
up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f

# Run all tests
test: test-python test-go test-ts

test-python:
	cd services/auth-service && pip install -r requirements.txt -q && pytest -v

test-go:
	cd services/rate-limiter && go test -v ./...

test-ts:
	cd services/notification-service && npm install --silent && npm test

# Run all linters
lint: lint-python lint-go lint-ts

lint-python:
	cd services/auth-service && flake8 --max-line-length=100 app.py test_app.py

lint-go:
	cd services/rate-limiter && go vet ./...

lint-ts:
	cd services/notification-service && npm run lint

# Build
build:
	docker compose build

# Clean up
clean:
	docker compose down -v --rmi local
	rm -rf services/notification-service/node_modules
	rm -rf services/notification-service/dist
	rm -rf services/rate-limiter/rate-limiter
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
