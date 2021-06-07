FROM golang:1.16.5-alpine3.13

WORKDIR /go/src/app
COPY . .
COPY   conf.json ./conf
RUN apk add make \
    && mkdir -p /data/log && make build-linux

EXPOSE 8899

CMD ["nohup","./bin/linux/push-service", ">> /data/log/push-service-console.log 2>&1 &"]