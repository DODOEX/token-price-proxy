FROM golang:alpine as builder

# ENV GOPROXY https://goproxy.cn,direct
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /app

COPY go.* ./
RUN go mod download
COPY . .

RUN apk update && apk add upx ca-certificates openssl && update-ca-certificates
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o /bin/tokenpriceproxy ./cmd/main.go
RUN upx -9 /bin/tokenpriceproxy

FROM gcr.io/distroless/static:nonroot
WORKDIR /app/
COPY --from=builder /bin/tokenpriceproxy /bin/tokenpriceproxy
COPY --from=builder --chown=nonroot /app/config /app/config
COPY --from=builder --chown=nonroot /app/storage /app/storage

EXPOSE 8080

ENTRYPOINT ["/bin/tokenpriceproxy"]