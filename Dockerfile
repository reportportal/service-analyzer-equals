FROM golang:1.11.1-alpine

WORKDIR /go/src/github.com/reportportal/service-analyzer-equals/
ARG version

RUN apk add --update --no-cache \
      build-base git curl \
      ca-certificates

## Copy makefile and glide before to be able to cache vendor
COPY Makefile ./
RUN make get-build-deps

COPY glide.yaml ./
COPY glide.lock ./

RUN make vendor

ENV VERSION=$version

RUN make get-build-deps
COPY ./ ./
RUN make checkstyle test build v=${VERSION}

FROM alpine:latest
ARG service
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/git.epam.com/reportportal/service-analyzer-tiger/bin/service-analyzer ./app
CMD ["./app"]
