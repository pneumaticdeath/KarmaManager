// go:build ios

#import <Foundation/Foundation.h>
#import <AVFoundation/AVFoundation.h>
#import <CoreVideo/CoreVideo.h>
#include <stdint.h>

int writeMP4ToPath(int w, int h, int n, int *delays_cs, uint8_t **frameData, const char *path) {
    NSString *outputPath = [NSString stringWithUTF8String:path];
    NSURL *outputURL = [NSURL fileURLWithPath:outputPath];

    // Remove any existing file at the destination.
    [[NSFileManager defaultManager] removeItemAtURL:outputURL error:nil];

    NSError *error = nil;
    AVAssetWriter *writer = [AVAssetWriter assetWriterWithURL:outputURL
                                                     fileType:AVFileTypeMPEG4
                                                        error:&error];
    if (!writer || error) {
        NSLog(@"AVAssetWriter creation failed: %@", error);
        return -1;
    }

    NSString *codecKey;
    if (@available(iOS 11.0, *)) {
        codecKey = AVVideoCodecTypeH264;
    } else {
        codecKey = AVVideoCodecH264;
    }
    NSDictionary *videoSettings = @{
        AVVideoCodecKey:  codecKey,
        AVVideoWidthKey:  @(w),
        AVVideoHeightKey: @(h),
    };

    AVAssetWriterInput *writerInput =
        [AVAssetWriterInput assetWriterInputWithMediaType:AVMediaTypeVideo
                                          outputSettings:videoSettings];
    writerInput.expectsMediaDataInRealTime = NO;

    // AVFoundation on iOS does not support 32RGBA for AVAssetWriter pixel buffers.
    // Use 32BGRA instead and swap R/B channels when copying the frame data.
    NSDictionary *pixelBufferAttributes = @{
        (NSString *)kCVPixelBufferPixelFormatTypeKey: @(kCVPixelFormatType_32BGRA),
        (NSString *)kCVPixelBufferWidthKey:           @(w),
        (NSString *)kCVPixelBufferHeightKey:          @(h),
    };

    AVAssetWriterInputPixelBufferAdaptor *adaptor =
        [AVAssetWriterInputPixelBufferAdaptor
            assetWriterInputPixelBufferAdaptorWithAssetWriterInput:writerInput
                                      sourcePixelBufferAttributes:pixelBufferAttributes];

    [writer addInput:writerInput];
    [writer startWriting];
    [writer startSessionAtSourceTime:kCMTimeZero];

    CMTime presentationTime = kCMTimeZero;

    for (int i = 0; i < n; i++) {
        // Wait until the input is ready to accept more data.
        while (!writerInput.isReadyForMoreMediaData) {
            [NSThread sleepForTimeInterval:0.005];
        }

        // Use the adaptor's pool so the pixel buffer format always matches.
        CVPixelBufferRef pixelBuffer = NULL;
        CVReturn cvRet = CVPixelBufferPoolCreatePixelBuffer(
            kCFAllocatorDefault, adaptor.pixelBufferPool, &pixelBuffer);
        if (cvRet != kCVReturnSuccess || pixelBuffer == NULL) {
            NSLog(@"CVPixelBufferPoolCreatePixelBuffer failed: %d", cvRet);
            return -1;
        }

        CVPixelBufferLockBaseAddress(pixelBuffer, 0);
        uint8_t *dest = (uint8_t *)CVPixelBufferGetBaseAddress(pixelBuffer);
        size_t bytesPerRow = CVPixelBufferGetBytesPerRow(pixelBuffer);
        uint8_t *src = frameData[i];
        // Convert RGBA → BGRA: swap bytes 0 (R) and 2 (B) in each pixel.
        for (int row = 0; row < h; row++) {
            uint8_t *d = dest + row * bytesPerRow;
            const uint8_t *s = src + row * w * 4;
            for (int col = 0; col < w; col++) {
                d[0] = s[2]; // B ← R
                d[1] = s[1]; // G
                d[2] = s[0]; // R ← B
                d[3] = s[3]; // A
                d += 4; s += 4;
            }
        }
        CVPixelBufferUnlockBaseAddress(pixelBuffer, 0);

        [adaptor appendPixelBuffer:pixelBuffer withPresentationTime:presentationTime];
        CVPixelBufferRelease(pixelBuffer);

        // Advance presentation time by this frame's delay (centiseconds → seconds).
        int delay_cs = delays_cs[i];
        if (delay_cs < 1) delay_cs = 1;
        CMTime frameDuration = CMTimeMake(delay_cs, 100);
        presentationTime = CMTimeAdd(presentationTime, frameDuration);
    }

    [writerInput markAsFinished];

    dispatch_semaphore_t sema = dispatch_semaphore_create(0);
    __block BOOL success = YES;
    [writer finishWritingWithCompletionHandler:^{
        if (writer.status != AVAssetWriterStatusCompleted) {
            NSLog(@"AVAssetWriter failed: %@", writer.error);
            success = NO;
        }
        dispatch_semaphore_signal(sema);
    }];
    dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);

    return success ? 0 : -1;
}
