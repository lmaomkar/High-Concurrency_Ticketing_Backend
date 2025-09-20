# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/reconcile ./cmd/reconcile
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/event-status-checker ./cmd/event-status-checker

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=builder /out/server /server
COPY --from=builder /out/worker /worker
COPY --from=builder /out/reconcile /reconcile
COPY --from=builder /out/event-status-checker /event-status-checker
COPY --from=builder /app/docs /docs
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]