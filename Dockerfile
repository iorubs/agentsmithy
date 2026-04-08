FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agentsmithy ./cmd/agentsmithy

FROM alpine:3.23
RUN apk add --no-cache git
COPY --from=builder /agentsmithy /usr/local/bin/agentsmithy
ENTRYPOINT ["agentsmithy"]
CMD ["serve"]
