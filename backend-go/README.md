# TransitOps Go backend

Requires Go 1.26.3 and PostgreSQL. Apply migrations with golang-migrate, then seed and run:

```powershell
$env:POSTGRES_DSN='postgres://transitops:transitops@localhost:5432/transitops?sslmode=disable'
$env:SECRET_KEY='local-dev-secret-key'
migrate -path migrations -database $env:POSTGRES_DSN up
go run ./cmd/seed
go run ./cmd/server
go vet ./...
go test ./...
golangci-lint run
```

Docker: `docker compose up --build postgres backend-go frontend`.
