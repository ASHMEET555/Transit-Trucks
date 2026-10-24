.PHONY: gen
gen:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 -generate types -package api -o internal/api/types.gen.go ../api/openapi.yaml
	npx --yes openapi-typescript ../api/openapi.yaml -o ../frontend/src/lib/api/schema.ts
