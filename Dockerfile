FROM alpine:latest

ADD out/adapter_linux_amd64 /usr/bin/adapter

RUN chmod +x /usr/bin/adapter

ADD launch.sh ./launch

CMD ./launch
