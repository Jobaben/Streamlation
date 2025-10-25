.PHONY: dev dev-api dev-web lint test

dev: dev-api
dev-api:
cd apps/api && go run ./cmd/server

dev-web:
pnpm --dir apps/web dev

lint:
pnpm lint && golangci-lint run ./...

test:
go test ./... && pnpm test
