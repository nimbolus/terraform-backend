# build binary
FROM --platform=$BUILDPLATFORM ghcr.io/crazy-max/xgo:latest AS builder

COPY . /source
WORKDIR /source

ARG TARGETPLATFORM
ENV TARGETS=$TARGETPLATFORM
ENV PACK=cmd/terraform-backend
ENV OUT=terraform-backend
ENV GO111MODULE=on
RUN xgo-build .

# start clean for final image
FROM debian:12

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -q update && \
  apt-get -yq install ca-certificates && \
  apt-get autoclean

ARG TARGETPLATFORM
COPY --from=builder /build/terraform-backend-${TARGETPLATFORM/\//-} /terraform-backend

ENTRYPOINT ["/terraform-backend"]
