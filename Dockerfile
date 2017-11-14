FROM ubuntu:16.04

ENV DEBIAN_FRONTEND noninteractive
ENV INITRD No

RUN dpkg-divert --local --rename --add /usr/bin/ischroot
RUN ln -sf /bin/true /usr/bin/ischroot
RUN dpkg-divert --local --rename --add /sbin/initctl
RUN ln -sf /bin/true /sbin/initctl

ADD build/bin/adapter /usr/bin/

RUN chmod +x /usr/bin/adapter

ADD launch.sh ./launch

EXPOSE 8080
CMD ./launch
