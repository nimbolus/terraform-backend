# build binary
FROM golang:1.17-alpine AS builder

COPY . /go/src/github.com/nimbolus/terraform-backend

WORKDIR /go/src/github.com/nimbolus/terraform-backend

RUN GOOS=linux CGO_ENABLED=0 go build -o terraform-backend

# start clean for final image
FROM alpine:3

COPY --from=builder /go/src/github.com/nimbolus/terraform-backend/terraform-backend /terraform-backend

ENTRYPOINT ["/terraform-backend"]
