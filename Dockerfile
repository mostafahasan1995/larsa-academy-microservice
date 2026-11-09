#first stage - builder
FROM golang:1.24 as builder

WORKDIR /app

ARG GITLAB_ACCESS_TOKEN
RUN git config --global url."https://oauth2:${GITLAB_ACCESS_TOKEN}@git.larsa.io/".insteadOf "https://git.larsa.io/"

COPY go.mod go.sum ./


ENV GO111MODULE=on

RUN CGO_ENABLED=0 GOOS=linux go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build main.go

#second stage
FROM debian:buster-slim
RUN echo "deb http://archive.debian.org/debian buster main contrib non-free" > /etc/apt/sources.list && \
    echo "deb http://archive.debian.org/debian-security buster/updates main" >> /etc/apt/sources.list && \
    apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/main .
# COPY --from=builder /app/.env .
COPY --from=builder /app/public/ ./public

CMD ["./main"]