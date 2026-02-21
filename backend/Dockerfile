FROM golang:1.26-bookworm AS base
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev ca-certificates && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download

FROM base AS build
COPY . .
RUN CGO_ENABLED=0 go build -o /durak-api ./cmd/api

FROM base AS test
COPY . .
CMD ["go", "test", "-race", "./..."]

FROM gcr.io/distroless/base-debian12
COPY --from=build /durak-api /usr/local/bin/durak-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/durak-api"]
