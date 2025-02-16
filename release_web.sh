#!/bin/bash

source .env

set -e

fyne package --target web
scp -r wasm/* "${WEBTARGET}"
