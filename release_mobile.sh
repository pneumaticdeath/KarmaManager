#!/bin/bash

set -e

source .env

rm -f KarmaManager.aab
fyne release --keystore ${ANDROIDKEYSTORE} --key-name "${ANDROIDKEYNAME}" --key-pass "${ANDROIDKEYPASS}" --target android

rm -f KarmaManager.ipa
fyne release --target ios --certificate "${IOSDISTROCERT}" --profile "${IOSDISTROPROF}"

# Post-process the IPA to inject missing Info.plist keys that Fyne doesn't support.
# Without NSPhotoLibraryAddUsageDescription, iOS hides "Save to Photos" in the share sheet.
echo "Injecting custom Info.plist keys into IPA..."
IPA_DIR=$(mktemp -d)
unzip -q KarmaManager.ipa -d "$IPA_DIR"

APP_DIR=$(find "$IPA_DIR/Payload" -name "*.app" -maxdepth 1)
PLIST="$APP_DIR/Info.plist"

/usr/libexec/PlistBuddy -c "Add :NSPhotoLibraryAddUsageDescription string 'Save animations to your photo library'" "$PLIST" 2>/dev/null \
    || /usr/libexec/PlistBuddy -c "Set :NSPhotoLibraryAddUsageDescription 'Save animations to your photo library'" "$PLIST"

# Extract existing entitlements and re-sign
ENTITLEMENTS=$(mktemp /tmp/entitlements.XXXXXX.plist)
codesign -d --entitlements "$ENTITLEMENTS" --xml "$APP_DIR" 2>/dev/null || true
codesign --force --sign "${IOSDISTROCERT}" --entitlements "$ENTITLEMENTS" --preserve-metadata=identifier,flags "$APP_DIR"
rm -f "$ENTITLEMENTS"

# Repack the IPA
rm -f KarmaManager.ipa
(cd "$IPA_DIR" && zip -qr - Payload) > KarmaManager.ipa
rm -rf "$IPA_DIR"
echo "Done — IPA updated with photo library usage description."
