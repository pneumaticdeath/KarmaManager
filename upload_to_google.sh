#!/bin/bash

set -e
source .env

VENV_DIR=".venv-upload"

if [ ! -d "$VENV_DIR" ]; then
    python3 -m venv "$VENV_DIR"
    "$VENV_DIR/bin/pip" install -q google-api-python-client google-auth
fi

"$VENV_DIR/bin/python3" upload_to_google.py
