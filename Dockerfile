FROM alpine:3.8

LABEL maintainer="Andrei Varabyeu <andrei_varabyeu@epam.com>"
LABEL version=4.0.1

ENV APP_DOWNLOAD_URL https://dl.bintray.com/epam/reportportal/4.0.1

ADD ${APP_DOWNLOAD_URL}/service-analyzer-equals_linux_amd64 /service-analyzer-equals

RUN chmod +x /service-analyzer-equals


EXPOSE 8080
ENTRYPOINT ["/service-analyzer-equals"]
