// go:build ios

#import <SafariServices/SFSafariViewController.h>
#import <UIKit/UIKit.h>

static SFSafariViewController *safariVC = nil;

void openOAuthBrowser(const char *url) {
    // Copy the URL string before the async dispatch since the C string may be freed.
    NSString *urlStr = [NSString stringWithUTF8String:url];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *nsURL = [NSURL URLWithString:urlStr];
        safariVC = [[SFSafariViewController alloc] initWithURL:nsURL];
        UIViewController *root =
            [UIApplication sharedApplication].keyWindow.rootViewController;
        [root presentViewController:safariVC animated:YES completion:nil];
    });
}

void dismissOAuthBrowser() {
    dispatch_async(dispatch_get_main_queue(), ^{
        [safariVC dismissViewControllerAnimated:YES completion:nil];
        safariVC = nil;
    });
}
