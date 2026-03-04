# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pb .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /pb
COPY --from=builder /app/pb .
COPY --from=builder /app/pb_public ./pb_public
EXPOSE 8090
CMD ["./pb", "serve", "--http=0.0.0.0:8090", "--dir=/pb/pb_data"]
