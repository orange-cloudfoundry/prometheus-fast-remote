FROM ubuntu:16.04

ENV DEBIAN_FRONTEND noninteractive
ENV INITRD No
ENV DEBIAN_PRIORITY critical
ENV DEBCONF_NOWARNINGS yes
ENV GOPATH /go
ENV GOBIN $GOPATH/bin
ENV GO15VENDOREXPERIMENT 1
ENV GO_VERSION 1.9.2
ENV GOROOT /usr/local/go
ENV PATH $PATH:$GOROOT/bin

RUN apt-get update
RUN apt-get install -y --no-install-recommends ca-certificates git wget
RUN cd /tmp && wget https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz && tar -xvf go${GO_VERSION}.linux-amd64.tar.gz && mv go /usr/local
RUN mkdir /go && mkdir /go/bin && mkdir /go/src
WORKDIR /go/src
ADD . /go/src/adapter
WORKDIR /go/src/adapter
CMD go install
