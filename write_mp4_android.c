#include <media/NdkMediaCodec.h>
#include <media/NdkMediaMuxer.h>
#include <media/NdkMediaFormat.h>
#include <android/log.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <fcntl.h>
#include <unistd.h>

#define LOG_TAG "KarmaManager"
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO,  LOG_TAG, __VA_ARGS__)
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)

// Convert RGBA to YUV420SemiPlanar (NV12: Y plane followed by interleaved UV plane).
static void rgbaToYUV420SP(const uint8_t *rgba, uint8_t *yuv, int w, int h) {
    int ySize = w * h;
    uint8_t *yPlane  = yuv;
    uint8_t *uvPlane = yuv + ySize;

    for (int j = 0; j < h; j++) {
        for (int i = 0; i < w; i++) {
            const uint8_t *p = rgba + (j * w + i) * 4;
            uint8_t r = p[0], g = p[1], b = p[2];

            // BT.601 limited range
            yPlane[j * w + i] = (uint8_t)((66 * r + 129 * g + 25 * b + 128) >> 8) + 16;

            if ((j & 1) == 0 && (i & 1) == 0) {
                int uvIdx = (j / 2) * w + i;
                uvPlane[uvIdx]     = (uint8_t)((-38 * r - 74 * g + 112 * b + 128) >> 8) + 128; // Cb
                uvPlane[uvIdx + 1] = (uint8_t)((112 * r - 94 * g -  18 * b + 128) >> 8) + 128; // Cr
            }
        }
    }
}

int writeMP4ToPath(int w, int h, int n, int *delays_cs, uint8_t **frameData, const char *path) {
    AMediaCodec *codec = AMediaCodec_createEncoderByType("video/avc");
    if (!codec) {
        LOGE("Failed to create AMediaCodec encoder");
        return -1;
    }

    AMediaFormat *format = AMediaFormat_new();
    AMediaFormat_setString(format, AMEDIAFORMAT_KEY_MIME,           "video/avc");
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_WIDTH,          w);
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_HEIGHT,         h);
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_BIT_RATE,       2000000); // 2 Mbps
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_FRAME_RATE,     15);
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_I_FRAME_INTERVAL, 1);
    AMediaFormat_setInt32(format,  AMEDIAFORMAT_KEY_COLOR_FORMAT,   21); // COLOR_FormatYUV420SemiPlanar

    media_status_t status = AMediaCodec_configure(codec, format, NULL, NULL,
                                                  AMEDIACODEC_CONFIGURE_FLAG_ENCODE);
    AMediaFormat_delete(format);
    if (status != AMEDIA_OK) {
        LOGE("AMediaCodec_configure failed: %d", status);
        AMediaCodec_delete(codec);
        return -1;
    }

    status = AMediaCodec_start(codec);
    if (status != AMEDIA_OK) {
        LOGE("AMediaCodec_start failed: %d", status);
        AMediaCodec_delete(codec);
        return -1;
    }

    int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
    if (fd < 0) {
        LOGE("Cannot open output file: %s", path);
        AMediaCodec_stop(codec);
        AMediaCodec_delete(codec);
        return -1;
    }

    AMediaMuxer *muxer = AMediaMuxer_new(fd, AMEDIAMUXER_OUTPUT_FORMAT_MPEG_4);
    if (!muxer) {
        LOGE("Failed to create AMediaMuxer");
        close(fd);
        AMediaCodec_stop(codec);
        AMediaCodec_delete(codec);
        return -1;
    }

    int yuvSize = w * h * 3 / 2;
    uint8_t *yuvBuf = (uint8_t *)malloc((size_t)yuvSize);
    if (!yuvBuf) {
        LOGE("OOM allocating YUV buffer");
        AMediaMuxer_delete(muxer);
        close(fd);
        AMediaCodec_stop(codec);
        AMediaCodec_delete(codec);
        return -1;
    }

    ssize_t trackIndex = -1;
    int muxerStarted = 0;
    int result = 0;
    int64_t presentationTimeUs = 0;

    // Feed all input frames plus an EOS marker, draining output along the way.
    for (int i = 0; i <= n; i++) {
        if (i < n) {
            ssize_t inputBufIdx = AMediaCodec_dequeueInputBuffer(codec, 100000 /* 100ms */);
            if (inputBufIdx >= 0) {
                size_t bufSize = 0;
                uint8_t *inputBuf = AMediaCodec_getInputBuffer(codec, (size_t)inputBufIdx, &bufSize);
                if (inputBuf) {
                    rgbaToYUV420SP(frameData[i], yuvBuf, w, h);
                    size_t copySize = (size_t)yuvSize < bufSize ? (size_t)yuvSize : bufSize;
                    memcpy(inputBuf, yuvBuf, copySize);

                    int delay_cs = delays_cs[i];
                    if (delay_cs < 1) delay_cs = 1;
                    AMediaCodec_queueInputBuffer(codec, (size_t)inputBufIdx, 0, copySize,
                                                 presentationTimeUs, 0);
                    presentationTimeUs += (int64_t)delay_cs * 10000; // centiseconds → microseconds
                }
            }
        } else {
            // Signal end of stream.
            ssize_t inputBufIdx = AMediaCodec_dequeueInputBuffer(codec, 100000);
            if (inputBufIdx >= 0) {
                AMediaCodec_queueInputBuffer(codec, (size_t)inputBufIdx, 0, 0,
                                             presentationTimeUs,
                                             AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM);
            }
        }

        // Drain available output buffers.
        AMediaCodecBufferInfo info;
        for (;;) {
            ssize_t outputBufIdx = AMediaCodec_dequeueOutputBuffer(codec, &info, 10000 /* 10ms */);
            if (outputBufIdx == AMEDIACODEC_INFO_TRY_AGAIN_LATER) {
                break;
            } else if (outputBufIdx == AMEDIACODEC_INFO_OUTPUT_FORMAT_CHANGED) {
                if (trackIndex < 0) {
                    AMediaFormat *newFormat = AMediaCodec_getOutputFormat(codec);
                    trackIndex = AMediaMuxer_addTrack(muxer, newFormat);
                    AMediaFormat_delete(newFormat);
                    AMediaMuxer_start(muxer);
                    muxerStarted = 1;
                }
            } else if (outputBufIdx >= 0) {
                if (muxerStarted && !(info.flags & AMEDIACODEC_BUFFER_FLAG_CODEC_CONFIG)) {
                    size_t outputSize = 0;
                    uint8_t *outputBuf = AMediaCodec_getOutputBuffer(codec, (size_t)outputBufIdx,
                                                                      &outputSize);
                    if (outputBuf && info.size > 0) {
                        AMediaMuxer_writeSampleData(muxer, (size_t)trackIndex, outputBuf, &info);
                    }
                }
                AMediaCodec_releaseOutputBuffer(codec, (size_t)outputBufIdx, 0);
                if (info.flags & AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM) {
                    goto done;
                }
            }
        }
    }

    // Extra drain in case EOS didn't surface in the interleaved loop above.
    for (int retry = 0; retry < 200; retry++) {
        AMediaCodecBufferInfo info;
        ssize_t outputBufIdx = AMediaCodec_dequeueOutputBuffer(codec, &info, 50000 /* 50ms */);
        if (outputBufIdx == AMEDIACODEC_INFO_TRY_AGAIN_LATER) {
            continue;
        } else if (outputBufIdx == AMEDIACODEC_INFO_OUTPUT_FORMAT_CHANGED) {
            if (trackIndex < 0) {
                AMediaFormat *newFormat = AMediaCodec_getOutputFormat(codec);
                trackIndex = AMediaMuxer_addTrack(muxer, newFormat);
                AMediaFormat_delete(newFormat);
                AMediaMuxer_start(muxer);
                muxerStarted = 1;
            }
        } else if (outputBufIdx >= 0) {
            if (muxerStarted && !(info.flags & AMEDIACODEC_BUFFER_FLAG_CODEC_CONFIG)) {
                size_t outputSize = 0;
                uint8_t *outputBuf = AMediaCodec_getOutputBuffer(codec, (size_t)outputBufIdx,
                                                                  &outputSize);
                if (outputBuf && info.size > 0) {
                    AMediaMuxer_writeSampleData(muxer, (size_t)trackIndex, outputBuf, &info);
                }
            }
            AMediaCodec_releaseOutputBuffer(codec, (size_t)outputBufIdx, 0);
            if (info.flags & AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM) {
                break;
            }
        }
    }

done:
    free(yuvBuf);
    if (muxerStarted) {
        AMediaMuxer_stop(muxer);
    }
    AMediaMuxer_delete(muxer);
    close(fd);
    AMediaCodec_stop(codec);
    AMediaCodec_delete(codec);

    return result;
}
