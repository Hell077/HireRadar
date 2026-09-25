FROM golang:1.27.0-alpine3.24 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd

FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app -u 10001 app
COPY --from=build --chown=10001:10001 /out/api /app/api
USER app
EXPOSE 8080
ENTRYPOINT ["/app/api"]
