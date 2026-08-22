#!/bin/bash -x

set -o errexit
set -o pipefail

FERNET_KEY_DIR="/etc/keystone/fernet-keys"

# Ensure Fernet keys are populated, check for 0 (staging) key
n=0
if [ ! -f "${FERNET_KEY_DIR}/0" ]; then
    sudo -H -u keystone keystone-manage db_sync

    keystone-manage --config-file /etc/keystone/keystone.conf fernet_setup \
        --keystone-user keystone --keystone-group keystone

    source /var/lib/kolla/config_files/admin-openrc.sh
    keystone-manage bootstrap --bootstrap-username "$OS_USERNAME" --bootstrap-password "$OS_PASSWORD" \
        --bootstrap-project-name "$OS_PROJECT_NAME" --bootstrap-role-name "admin" \
        --bootstrap-internal-url "$OS_AUTH_URL" --bootstrap-public-url "$OS_AUTH_URL" \
        --bootstrap-service-name "keystone" --bootstrap-region-id "$OS_REGION_NAME"
fi

exec /usr/sbin/httpd $@
