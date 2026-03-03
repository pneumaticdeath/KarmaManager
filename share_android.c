#include <jni.h>
#include <android/log.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

#define LOG_TAG "KarmaManager"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)

// Insert the GIF into MediaStore.Downloads (API 29+) and return a content:// URI.
// Returns NULL on failure.
static jobject insertIntoMediaStore(JNIEnv *env, jobject activity, const char *path) {
    // Get ContentResolver
    jclass contextClass = (*env)->GetObjectClass(env, activity);
    jmethodID getContentResolver = (*env)->GetMethodID(env, contextClass, "getContentResolver",
        "()Landroid/content/ContentResolver;");
    jobject resolver = (*env)->CallObjectMethod(env, activity, getContentResolver);

    // Build ContentValues
    jclass cvClass = (*env)->FindClass(env, "android/content/ContentValues");
    jmethodID cvInit = (*env)->GetMethodID(env, cvClass, "<init>", "()V");
    jobject cv = (*env)->NewObject(env, cvClass, cvInit);
    jmethodID putStr = (*env)->GetMethodID(env, cvClass, "put",
        "(Ljava/lang/String;Ljava/lang/String;)V");

    const char *filename = strrchr(path, '/');
    filename = filename ? filename + 1 : path;

    jstring keyName = (*env)->NewStringUTF(env, "_display_name");
    jstring jFilename = (*env)->NewStringUTF(env, filename);
    (*env)->CallVoidMethod(env, cv, putStr, keyName, jFilename);

    jstring keyMime = (*env)->NewStringUTF(env, "mime_type");
    jstring jMime = (*env)->NewStringUTF(env, "image/gif");
    (*env)->CallVoidMethod(env, cv, putStr, keyMime, jMime);

    // MediaStore.Downloads.EXTERNAL_CONTENT_URI
    jclass downloadsClass = (*env)->FindClass(env, "android/provider/MediaStore$Downloads");
    if (downloadsClass == NULL || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("MediaStore.Downloads not available (API < 29)");
        return NULL;
    }
    jfieldID uriField = (*env)->GetStaticFieldID(env, downloadsClass,
        "EXTERNAL_CONTENT_URI", "Landroid/net/Uri;");
    jobject externalUri = (*env)->GetStaticObjectField(env, downloadsClass, uriField);

    // Insert to get item URI
    jclass resolverClass = (*env)->GetObjectClass(env, resolver);
    jmethodID insertMethod = (*env)->GetMethodID(env, resolverClass, "insert",
        "(Landroid/net/Uri;Landroid/content/ContentValues;)Landroid/net/Uri;");
    jobject itemUri = (*env)->CallObjectMethod(env, resolver, insertMethod, externalUri, cv);
    if (itemUri == NULL || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("MediaStore insert failed");
        return NULL;
    }

    // Open OutputStream and write file bytes
    jmethodID openOutput = (*env)->GetMethodID(env, resolverClass, "openOutputStream",
        "(Landroid/net/Uri;)Ljava/io/OutputStream;");
    jobject outputStream = (*env)->CallObjectMethod(env, resolver, openOutput, itemUri);
    if (outputStream == NULL || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("openOutputStream failed");
        return NULL;
    }

    FILE *f = fopen(path, "rb");
    if (f == NULL) { LOGE("Cannot open source file: %s", path); return NULL; }

    jclass osClass = (*env)->GetObjectClass(env, outputStream);
    jmethodID writeMethod = (*env)->GetMethodID(env, osClass, "write", "([BII)V");
    jmethodID closeMethod = (*env)->GetMethodID(env, osClass, "close", "()V");

    jbyteArray buf = (*env)->NewByteArray(env, 8192);
    char cbuf[8192];
    size_t n;
    while ((n = fread(cbuf, 1, sizeof(cbuf), f)) > 0) {
        (*env)->SetByteArrayRegion(env, buf, 0, (jsize)n, (jbyte *)cbuf);
        (*env)->CallVoidMethod(env, outputStream, writeMethod, buf, 0, (jint)n);
        if ((*env)->ExceptionCheck(env)) {
            (*env)->ExceptionClear(env);
            LOGE("write failed");
            break;
        }
    }
    fclose(f);
    (*env)->CallVoidMethod(env, outputStream, closeMethod);

    return itemUri;
}

void shareGIFViaJNI(uintptr_t envPtr, uintptr_t ctxPtr, const char *path) {
    JNIEnv *env = (JNIEnv *)envPtr;
    jobject activity = (jobject)ctxPtr;

    jobject uri = insertIntoMediaStore(env, activity, path);
    if (uri == NULL) {
        LOGE("Could not get shareable URI for GIF");
        return;
    }

    // Build ACTION_SEND intent
    jclass intentClass = (*env)->FindClass(env, "android/content/Intent");
    jmethodID intentInit = (*env)->GetMethodID(env, intentClass, "<init>",
        "(Ljava/lang/String;)V");
    jstring actionSend = (*env)->NewStringUTF(env, "android.intent.action.SEND");
    jobject intent = (*env)->NewObject(env, intentClass, intentInit, actionSend);

    jmethodID setType = (*env)->GetMethodID(env, intentClass, "setType",
        "(Ljava/lang/String;)Landroid/content/Intent;");
    jstring mimeType = (*env)->NewStringUTF(env, "image/gif");
    (*env)->CallObjectMethod(env, intent, setType, mimeType);

    jmethodID putExtra = (*env)->GetMethodID(env, intentClass, "putExtra",
        "(Ljava/lang/String;Landroid/os/Parcelable;)Landroid/content/Intent;");
    jstring extraStream = (*env)->NewStringUTF(env, "android.intent.extra.STREAM");
    (*env)->CallObjectMethod(env, intent, putExtra, extraStream, uri);

    jmethodID addFlags = (*env)->GetMethodID(env, intentClass, "addFlags",
        "(I)Landroid/content/Intent;");
    (*env)->CallObjectMethod(env, intent, addFlags, (jint)1); // FLAG_GRANT_READ_URI_PERMISSION

    // Wrap in chooser
    jmethodID createChooser = (*env)->GetStaticMethodID(env, intentClass, "createChooser",
        "(Landroid/content/Intent;Ljava/lang/CharSequence;)Landroid/content/Intent;");
    jstring chooserTitle = (*env)->NewStringUTF(env, "Share GIF");
    jobject chooser = (*env)->CallStaticObjectMethod(env, intentClass,
        createChooser, intent, chooserTitle);

    // startActivity
    jclass activityClass = (*env)->GetObjectClass(env, activity);
    jmethodID startActivity = (*env)->GetMethodID(env, activityClass, "startActivity",
        "(Landroid/content/Intent;)V");
    (*env)->CallVoidMethod(env, activity, startActivity, chooser);

    LOGI("Share intent launched for %s", path);
}
