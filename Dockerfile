# syntax=docker/dockerfile:1

FROM golang:latest as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/autodev ./cmd/main/

FROM scratch

COPY --from=builder /bin/autodev /bin/autodev

EXPOSE 8080

CMD ["/bin/autodev"]

