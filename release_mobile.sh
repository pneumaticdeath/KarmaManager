#!/bin/bash

set -e

source .env 

rm -f KarmaManager.aab
fyne release --key-store ${ANDROIDKEYSTORE} --key-name "${ANDROIDKEYNAME}" --key-pass "${ANDROIDKEYPASS}" --target android

rm -f KarmaManager.ipa
fyne release --target ios --certificate "${IOSDISTROCERT}" --profile "${IOSDISTROPROF}"
