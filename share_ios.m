// go:build ios

#import <UIKit/UIKit.h>
#import <Foundation/Foundation.h>

void shareGIFFile(const char *path) {
    NSURL *fileURL = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path]];
    UIActivityViewController *avc =
        [[UIActivityViewController alloc]
            initWithActivityItems:@[fileURL]
            applicationActivities:nil];
    UIViewController *rootVC =
        [UIApplication sharedApplication].keyWindow.rootViewController;
    // iPad requires popover anchor or crashes
    if (avc.popoverPresentationController) {
        avc.popoverPresentationController.sourceView = rootVC.view;
        avc.popoverPresentationController.sourceRect =
            CGRectMake(rootVC.view.bounds.size.width / 2,
                       rootVC.view.bounds.size.height, 1, 1);
    }
    [rootVC presentViewController:avc animated:YES completion:nil];
}
