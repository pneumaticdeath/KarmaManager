#!/usr/bin/env python3
"""Upload KarmaManager.aab to Google Play internal test track."""

import os
import sys

from google.oauth2 import service_account
from googleapiclient.discovery import build
from googleapiclient.http import MediaFileUpload

PACKAGE_NAME = "io.patenaude.karmamanager"
AAB_PATH = os.environ.get("GOOGLE_PLAY_AAB", "KarmaManager.aab")
KEY_FILE = os.environ.get("GOOGLE_PLAY_KEY_FILE", "")
TRACK = os.environ.get("GOOGLE_PLAY_TRACK", "internal")

def main():
    if not KEY_FILE:
        print("Error: GOOGLE_PLAY_KEY_FILE not set in environment")
        sys.exit(1)
    if not os.path.exists(KEY_FILE):
        print(f"Error: key file not found: {KEY_FILE}")
        sys.exit(1)
    if not os.path.exists(AAB_PATH):
        print(f"Error: AAB not found: {AAB_PATH}")
        sys.exit(1)

    credentials = service_account.Credentials.from_service_account_file(
        KEY_FILE,
        scopes=["https://www.googleapis.com/auth/androidpublisher"],
    )
    service = build("androidpublisher", "v3", credentials=credentials)

    # Create a new edit
    edit = service.edits().insert(body={}, packageName=PACKAGE_NAME).execute()
    edit_id = edit["id"]
    print(f"Created edit {edit_id}")

    # Upload the AAB
    print(f"Uploading {AAB_PATH} ...")
    media = MediaFileUpload(AAB_PATH, mimetype="application/octet-stream")
    bundle = (
        service.edits()
        .bundles()
        .upload(packageName=PACKAGE_NAME, editId=edit_id, media_body=media)
        .execute()
    )
    version_code = bundle["versionCode"]
    print(f"Uploaded bundle version code: {version_code}")

    # Assign to track
    # Use "draft" status for apps not yet published; "completed" for published apps.
    status = os.environ.get("GOOGLE_PLAY_STATUS", "draft")
    service.edits().tracks().update(
        packageName=PACKAGE_NAME,
        editId=edit_id,
        track=TRACK,
        body={
            "track": TRACK,
            "releases": [
                {
                    "versionCodes": [str(version_code)],
                    "status": status,
                }
            ],
        },
    ).execute()
    print(f"Assigned to '{TRACK}' track with status '{status}'")

    # Commit the edit
    service.edits().commit(packageName=PACKAGE_NAME, editId=edit_id).execute()
    print("Edit committed — upload complete!")


if __name__ == "__main__":
    main()
