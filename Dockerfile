# build binary
FROM golang:1.22 AS builder

COPY . /go/src/github.com/nimbolus/terraform-backend

WORKDIR /go/src/github.com/nimbolus/terraform-backend

RUN GOOS=linux CGO_ENABLED=1 go build ./cmd/terraform-backend

# start clean for final image
FROM debian:12

RUN apt-get -q update && \
  apt-get -yq install ca-certificates && \
  apt-get autoclean

COPY --from=builder /go/src/github.com/nimbolus/terraform-backend/terraform-backend /terraform-backend

ENTRYPOINT ["/terraform-backend"]
