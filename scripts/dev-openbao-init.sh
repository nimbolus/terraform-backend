#!/bin/sh

export TOKEN=$1
echo "TOKEN: $TOKEN"

bao login -method=token token=$TOKEN

bao secrets enable transit
bao write transit/keys/terraform-backend exportable=true

sleep infinity
