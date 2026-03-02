#include <jni.h>
#include <android/log.h>
#include <stdlib.h>
#include <string.h>

#define LOG_TAG "KarmaManager"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)

void shareGIFViaJNI(uintptr_t envPtr, uintptr_t ctxPtr, const char *path) {
    JNIEnv *env = (JNIEnv *)envPtr;
    jobject activity = (jobject)ctxPtr;

    // Create File object from path
    jclass fileClass = (*env)->FindClass(env, "java/io/File");
    if (fileClass == NULL) { LOGE("File class not found"); return; }
    jmethodID fileConstructor = (*env)->GetMethodID(env, fileClass, "<init>", "(Ljava/lang/String;)V");
    jstring jpath = (*env)->NewStringUTF(env, path);
    jobject fileObj = (*env)->NewObject(env, fileClass, fileConstructor, jpath);

    // Get URI via FileProvider.getUriForFile(context, authority, file)
    jclass fileProviderClass = (*env)->FindClass(env, "androidx/core/content/FileProvider");
    if (fileProviderClass == NULL) { LOGE("FileProvider class not found"); return; }
    jmethodID getUriMethod = (*env)->GetStaticMethodID(env, fileProviderClass,
        "getUriForFile",
        "(Landroid/content/Context;Ljava/lang/String;Ljava/io/File;)Landroid/net/Uri;");
    if (getUriMethod == NULL) { LOGE("getUriForFile method not found"); return; }
    jstring authority = (*env)->NewStringUTF(env, "io.patenaude.karmamanager.fileprovider");
    jobject uri = (*env)->CallStaticObjectMethod(env, fileProviderClass, getUriMethod,
        activity, authority, fileObj);
    if (uri == NULL) { LOGE("getUriForFile returned null"); return; }

    // Build ACTION_SEND intent
    jclass intentClass = (*env)->FindClass(env, "android/content/Intent");
    jmethodID intentConstructor = (*env)->GetMethodID(env, intentClass, "<init>", "(Ljava/lang/String;)V");
    jstring actionSend = (*env)->NewStringUTF(env, "android.intent.action.SEND");
    jobject intent = (*env)->NewObject(env, intentClass, intentConstructor, actionSend);

    // setType("image/gif")
    jmethodID setTypeMethod = (*env)->GetMethodID(env, intentClass, "setType",
        "(Ljava/lang/String;)Landroid/content/Intent;");
    jstring mimeType = (*env)->NewStringUTF(env, "image/gif");
    (*env)->CallObjectMethod(env, intent, setTypeMethod, mimeType);

    // putExtra(EXTRA_STREAM, uri)
    jmethodID putExtraMethod = (*env)->GetMethodID(env, intentClass, "putExtra",
        "(Ljava/lang/String;Landroid/os/Parcelable;)Landroid/content/Intent;");
    jstring extraStream = (*env)->NewStringUTF(env, "android.intent.extra.STREAM");
    (*env)->CallObjectMethod(env, intent, putExtraMethod, extraStream, uri);

    // addFlags(FLAG_GRANT_READ_URI_PERMISSION = 1)
    jmethodID addFlagsMethod = (*env)->GetMethodID(env, intentClass, "addFlags",
        "(I)Landroid/content/Intent;");
    (*env)->CallObjectMethod(env, intent, addFlagsMethod, (jint)1);

    // Intent.createChooser(intent, "Share GIF")
    jmethodID createChooserMethod = (*env)->GetStaticMethodID(env, intentClass, "createChooser",
        "(Landroid/content/Intent;Ljava/lang/CharSequence;)Landroid/content/Intent;");
    jstring chooserTitle = (*env)->NewStringUTF(env, "Share GIF");
    jobject chooserIntent = (*env)->CallStaticObjectMethod(env, intentClass,
        createChooserMethod, intent, chooserTitle);

    // activity.startActivity(chooserIntent)
    jclass activityClass = (*env)->GetObjectClass(env, activity);
    jmethodID startActivityMethod = (*env)->GetMethodID(env, activityClass, "startActivity",
        "(Landroid/content/Intent;)V");
    (*env)->CallVoidMethod(env, activity, startActivityMethod, chooserIntent);

    // Clean up local references
    (*env)->DeleteLocalRef(env, jpath);
    (*env)->DeleteLocalRef(env, fileObj);
    (*env)->DeleteLocalRef(env, authority);
    (*env)->DeleteLocalRef(env, uri);
    (*env)->DeleteLocalRef(env, actionSend);
    (*env)->DeleteLocalRef(env, intent);
    (*env)->DeleteLocalRef(env, mimeType);
    (*env)->DeleteLocalRef(env, extraStream);
    (*env)->DeleteLocalRef(env, chooserTitle);
    (*env)->DeleteLocalRef(env, chooserIntent);
}
