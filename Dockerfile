FROM alpine:latest

ADD out/adapter /usr/bin/

RUN chmod +x /usr/bin/adapter

ADD launch.sh ./launch

EXPOSE 8080
CMD ./launch
