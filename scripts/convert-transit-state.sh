#?/usr/bin/env bash

# This scripts converts a state which was encrypted with the Vault Transit engine
# so that it can be uncrypted by the local KMS module

STATE_FILE=$1

if [ -z "$STATE_FILE" ]; then
    echo "no state file defined: usage ./convert-transit-state.sh <state-file>"
    exit 1
fi

echo "creating state file backup at $STATE_FILE.backup"
cp $STATE_FILE $STATE_FILE.backup

echo "converting state file"
sed 's/vault:v1:*//' $STATE_FILE.backup | base64 -d > $STATE_FILE
