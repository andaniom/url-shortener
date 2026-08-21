FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /lnk -ldflags="-s -w" .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /lnk .
COPY static/ ./static/

RUN mkdir -p /app/data

EXPOSE 8080

ENV TZ=Asia/Jakarta
ENV DB_PATH=/app/data/lnk.db

CMD ["./lnk"]
