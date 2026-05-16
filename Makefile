.PHONY: build dev frontend clean docker

# Build everything (frontend + Go binary)
build: frontend
	go build -o atom2api .

# Build frontend only
frontend:
	cd frontend && npm run build

# Dev mode: Go backend with hot reload
dev:
	go run .

# Dev mode: Frontend dev server (proxies to Go backend)
frontend-dev:
	cd frontend && npm run dev

# Docker build
docker:
	docker compose build

# Docker run
up:
	docker compose up -d

# Clean build artifacts
clean:
	rm -f atom2api atom2api.exe
	rm -rf frontend/dist
	rm -rf data

# Initialize git repo
init:
	git init
	git add -A
	git commit -m "Initial commit: Atom2API"
