FROM golang:1.17 as builder

WORKDIR /app

COPY . .

RUN GOPROXY=https://goproxy.cn CGO_ENABLED=0 go build -o ingress-manager main.go

FROM pee6w651.mirror.aliyuncs.com/library/alpine:latest

WORKDIR /app

COPY --from=builder /app/ingress-manager .

CMD ["./ingress-manager"]