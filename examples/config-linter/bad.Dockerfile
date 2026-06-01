# Issues: latest tag, MAINTAINER, sudo, apt-get upgrade, ADD instead of COPY, no USER
FROM ubuntu:latest
MAINTAINER dev@example.com
RUN sudo apt-get update && apt-get upgrade -y && apt-get install -y curl
ADD app.tar.gz /app/
WORKDIR app
CMD ["/app/app"]
