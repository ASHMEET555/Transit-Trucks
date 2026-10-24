FROM golang:1.26.3 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/transitops-go ./cmd/server
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/transitops-go /transitops-go
EXPOSE 8000
ENTRYPOINT ["/transitops-go"]
