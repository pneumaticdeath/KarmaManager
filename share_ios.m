// go:build ios

#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>

void shareGIFFile(const char *path) {
    // Copy path before the async dispatch since the C string may be freed.
    NSString *filePath = [NSString stringWithUTF8String:path];

    // UIKit requires all view operations on the main thread.
    // fyne.Do runs on Fyne's event goroutine, not the iOS main thread.
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *fileURL = [NSURL fileURLWithPath:filePath];
        UIViewController *rootVC =
            [UIApplication sharedApplication].keyWindow.rootViewController;

#if TARGET_OS_SIMULATOR
        // Simulator share sheet has no useful destinations; use the Files
        // document picker so the GIF can actually be saved somewhere.
        UIDocumentPickerViewController *picker;
        if (@available(iOS 14.0, *)) {
            picker = [[UIDocumentPickerViewController alloc]
                          initForExportingURLs:@[fileURL]];
        } else {
            picker = [[UIDocumentPickerViewController alloc]
                          initWithURL:fileURL
                               inMode:UIDocumentPickerModeExportToService];
        }
        if (picker.popoverPresentationController) {
            picker.popoverPresentationController.sourceView = rootVC.view;
            picker.popoverPresentationController.sourceRect =
                CGRectMake(rootVC.view.bounds.size.width / 2,
                           rootVC.view.bounds.size.height, 1, 1);
        }
        [rootVC presentViewController:picker animated:YES completion:nil];
#else
        // Real device: full share sheet.
        UIActivityViewController *avc =
            [[UIActivityViewController alloc]
                initWithActivityItems:@[fileURL]
                applicationActivities:nil];
        // iPad requires popover anchor or crashes.
        if (avc.popoverPresentationController) {
            avc.popoverPresentationController.sourceView = rootVC.view;
            avc.popoverPresentationController.sourceRect =
                CGRectMake(rootVC.view.bounds.size.width / 2,
                           rootVC.view.bounds.size.height, 1, 1);
        }
        [rootVC presentViewController:avc animated:YES completion:nil];
#endif
    });
}
