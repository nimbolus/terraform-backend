# build binary
FROM golang:1.27-alpine AS builder

COPY . /go/src/github.com/nimbolus/terraform-backend

WORKDIR /go/src/github.com/nimbolus/terraform-backend

# version written to the metadata of stored states
ARG VERSION=""

RUN GOOS=linux CGO_ENABLED=0 go build -ldflags "-X github.com/nimbolus/terraform-backend/pkg/version.Version=${VERSION}" cmd/terraform-backend.go

# start clean for final image
FROM alpine:3

COPY --from=builder /go/src/github.com/nimbolus/terraform-backend/terraform-backend /terraform-backend

ENTRYPOINT ["/terraform-backend"]
