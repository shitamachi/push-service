FROM golang:1.16

WORKDIR /go/src/app
COPY . .
COPY   conf.json ./conf
RUN mkdir -p /data/log && make build-linux

EXPOSE 8899

CMD ["nohup","./bin/linux/push-service", ">> /data/log/push-service-console.log 2>&1 &"]