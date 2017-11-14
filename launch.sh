#!/bin/sh

set -e

cat << EOF > config.yml
kairos_url: ${KAIROS}
skip_insecure: ${SKIP_INSECURE:-false}
listen_addr: 0.0.0.0:8080
log_level: ${LOG_LEVEL:-info}
log_json: ${LOG_JSON:-true}
no_color: ${NO_COLOR:-false}
workers: ${WORKERS:-5}
EOF

/usr/bin/adapter
