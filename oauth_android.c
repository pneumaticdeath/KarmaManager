#include <jni.h>
#include <android/log.h>
#include <stdlib.h>

#define LOG_TAG "KarmaManager"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)

void openOAuthBrowserJNI(uintptr_t envPtr, uintptr_t ctxPtr, const char *url) {
    JNIEnv *env = (JNIEnv *)envPtr;
    jobject activity = (jobject)ctxPtr;

    // Uri.parse(url)
    jclass uriClass = (*env)->FindClass(env, "android/net/Uri");
    if (!uriClass || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("Uri class not found");
        return;
    }
    jmethodID parseMethod = (*env)->GetStaticMethodID(env, uriClass, "parse",
        "(Ljava/lang/String;)Landroid/net/Uri;");
    jstring jurl = (*env)->NewStringUTF(env, url);
    jobject uri = (*env)->CallStaticObjectMethod(env, uriClass, parseMethod, jurl);
    if (!uri || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("Uri.parse failed");
        return;
    }

    // new Intent(Intent.ACTION_VIEW, uri)
    jclass intentClass = (*env)->FindClass(env, "android/content/Intent");
    jmethodID intentInit = (*env)->GetMethodID(env, intentClass, "<init>",
        "(Ljava/lang/String;Landroid/net/Uri;)V");
    jstring actionView = (*env)->NewStringUTF(env, "android.intent.action.VIEW");
    jobject intent = (*env)->NewObject(env, intentClass, intentInit, actionView, uri);
    if (!intent || (*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("Intent creation failed");
        return;
    }

    // activity.startActivity(intent)
    jclass activityClass = (*env)->GetObjectClass(env, activity);
    jmethodID startActivity = (*env)->GetMethodID(env, activityClass, "startActivity",
        "(Landroid/content/Intent;)V");
    (*env)->CallVoidMethod(env, activity, startActivity, intent);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        LOGE("startActivity failed");
        return;
    }

    LOGI("OAuth browser opened");
}
