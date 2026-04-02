#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source .env

MACOS_MIN_VERSION="12.0"
MACOSAPPSTORECERT="3rd Party Mac Developer Application: Mitchell Ross Patenaude (W245SMS7LR)"
MACOSINSTALLERCERT="3rd Party Mac Developer Installer: Mitchell Ross Patenaude (W245SMS7LR)"
MACOSAPPSTOREPROF="${HOME}/.xcode_profiles/KarmaManager_Mac_App_Store.provisionprofile"

echo "==> Building with fyne release..."
rm -f KarmaManager.pkg
export MACOSX_DEPLOYMENT_TARGET=${MACOS_MIN_VERSION}
export CGO_CFLAGS="-mmacosx-version-min=${MACOS_MIN_VERSION}"
export CGO_LDFLAGS="-mmacosx-version-min=${MACOS_MIN_VERSION}"
fyne release --target darwin \
    --certificate "${MACOSAPPSTORECERT}" \
    --profile "${MACOSAPPSTOREPROF}" \
    --category games

echo "==> Extracting .app from .pkg..."
WORK=$(mktemp -d)
pkgutil --expand KarmaManager.pkg "${WORK}/expanded"
mkdir -p "${WORK}/payload"
(cd "${WORK}/payload" && gunzip -c "../expanded/io.patenaude.karmamanager.pkg/Payload" | cpio -id 2>/dev/null)

echo "==> Patching Info.plist (LSMinimumSystemVersion -> ${MACOS_MIN_VERSION})..."
/usr/libexec/PlistBuddy -c "Set :LSMinimumSystemVersion ${MACOS_MIN_VERSION}" \
    "${WORK}/payload/KarmaManager.app/Contents/Info.plist"

echo "==> Embedding provisioning profile..."
cp "${MACOSAPPSTOREPROF}" "${WORK}/payload/KarmaManager.app/Contents/embedded.provisionprofile"
xattr -cr "${WORK}/payload/KarmaManager.app"

echo "==> Re-signing with entitlements and sandbox..."
codesign --force --deep \
    --sign "${MACOSAPPSTORECERT}" \
    --entitlements "${SCRIPT_DIR}/macOS-sandbox-entitlements.xml" \
    "${WORK}/payload/KarmaManager.app"

echo "==> Building installer .pkg with productbuild..."
rm -f KarmaManager.pkg
productbuild \
    --component "${WORK}/payload/KarmaManager.app" /Applications \
    --sign "${MACOSINSTALLERCERT}" \
    KarmaManager.pkg

rm -rf "${WORK}"

echo "==> Validating with App Store Connect..."
xcrun altool --validate-app \
    -f KarmaManager.pkg \
    -t macos \
    -u "${IOSUPLOADUSER}" \
    -p "${IOSUPLOADPASS}"

echo "==> Uploading to App Store Connect..."
xcrun altool --upload-app \
    -f KarmaManager.pkg \
    -t macos \
    -u "${IOSUPLOADUSER}" \
    -p "${IOSUPLOADPASS}"

echo "==> Done! Check App Store Connect for processing status."
