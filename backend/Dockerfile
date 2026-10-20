# Multi-stage Dockerfile for Go API
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN sed -i 's/^go .*/go 1.24.0/' go.mod && go mod download

COPY . ./
RUN sed -i 's/^go .*/go 1.24.0/' go.mod && CGO_ENABLED=0 GOOS=linux go build -o /tripledger .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/
COPY --from=builder /tripledger .

EXPOSE 8080
CMD ["./tripledger"]
