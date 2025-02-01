#!/bin/bash

set -e

source .env 

rm -f KarmaManager.aab
fyne release --keyStore ${ANDROIDKEYSTORE} --keyName "${ANDROIDKEYNAME}" --target android

rm -f KarmaManager.ipa
fyne release --target ios --certificate "${IOSDISTROCERT}" --profile "${IOSDISTROPROF}"
