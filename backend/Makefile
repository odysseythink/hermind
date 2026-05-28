.PHONY: build build-frontend build-server dev test lint

FRONTEND_DIST := frontend/dist
GOFLAGS := -tags="fts5"

build-frontend:
	cd ../frontend && yarn install && yarn build

build-server: build-frontend
	cp -r ../frontend/dist ./frontend/dist
	mv ./frontend/dist/_index.html ./frontend/dist/index.html
	go build $(GOFLAGS) -o ../anything-llm ./cmd/server/

dev:
	go run $(GOFLAGS) ./cmd/server/ -logtostderr

test:
	go test -v ./...

lint:
	golangci-lint run ./...
