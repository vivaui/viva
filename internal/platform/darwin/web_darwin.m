#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <WebKit/WebKit.h>

#include <stdlib.h>
#include <string.h>

#include "web_darwin.h"

// What the page does reaches Go through these (web_darwin.go).
extern void vivaWebMessage(uintptr_t handle, uint32_t id, char *utf8);
extern void vivaWebLoad(uintptr_t handle, uint32_t id, int phase, char *url);
extern int vivaWebServe(uintptr_t handle, uint32_t id, char *scheme, char *path, char **mime,
                        unsigned char **data, size_t *length);
extern void vivaWebReady(uintptr_t handle);

static void viva_web_main(void (^work)(void)) {
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
}

// VivaWebBridge is everything the page may call: its script messages, its navigation, and requests
// to its custom scheme.
@interface VivaWebBridge : NSObject <WKScriptMessageHandler, WKNavigationDelegate, WKURLSchemeHandler> {
@public
    uintptr_t _handle;
    uint32_t _id;
}
@end

@implementation VivaWebBridge

- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
    (void)controller;
    if (![message.body isKindOfClass:[NSString class]]) {
        return;
    }
    vivaWebMessage(_handle, _id, (char *)[(NSString *)message.body UTF8String]);
}

- (void)notify:(WKWebView *)web phase:(int)phase {
    NSString *address = web.URL.absoluteString;
    vivaWebLoad(_handle, _id, phase, (char *)(address != nil ? address.UTF8String : ""));
}

- (void)webView:(WKWebView *)web didStartProvisionalNavigation:(WKNavigation *)navigation {
    (void)navigation;
    [self notify:web phase:0];
}

- (void)webView:(WKWebView *)web didCommitNavigation:(WKNavigation *)navigation {
    (void)navigation;
    [self notify:web phase:1];
}

- (void)webView:(WKWebView *)web didFinishNavigation:(WKNavigation *)navigation {
    (void)navigation;
    [self notify:web phase:2];
}

- (void)webView:(WKWebView *)web didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    (void)navigation;
    (void)error;
    [self notify:web phase:2];
}

- (void)webView:(WKWebView *)web didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    (void)navigation;
    (void)error;
    [self notify:web phase:2];
}

- (void)webView:(WKWebView *)web startURLSchemeTask:(id<WKURLSchemeTask>)task {
    (void)web;
    NSURL *url = task.request.URL;
    NSString *path = url.path != nil ? url.path : @"";
    if ([path hasPrefix:@"/"]) {
        path = [path substringFromIndex:1];
    }
    char *mime = NULL;
    unsigned char *data = NULL;
    size_t length = 0;
    int found = vivaWebServe(_handle, _id, (char *)url.scheme.UTF8String, (char *)path.UTF8String, &mime, &data, &length);
    NSDictionary *headers = @{
        @"Content-Type": found && mime != NULL ? [NSString stringWithUTF8String:mime] : @"text/plain",
        @"Content-Length": [NSString stringWithFormat:@"%zu", found ? length : (size_t)0],
        @"Access-Control-Allow-Origin": @"*",
    };
    NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:url
                                                              statusCode:found ? 200 : 404
                                                             HTTPVersion:@"HTTP/1.1"
                                                            headerFields:headers];
    [task didReceiveResponse:response];
    [response release];
    if (found && length > 0) {
        [task didReceiveData:[NSData dataWithBytes:data length:length]];
    }
    free(mime);
    free(data);
    [task didFinish];
}

- (void)webView:(WKWebView *)web stopURLSchemeTask:(id<WKURLSchemeTask>)task {
    (void)web;
    (void)task;
}

@end

// VivaWebView is a page that lies over Viva's drawing and gives way where Viva draws over it: the
// parts in cuts are masked off, so Viva's picture shows, and do not take the pointer, so Viva does.
@interface VivaWebView : WKWebView {
    NSArray<NSValue *> *_cuts;   // the parts that refuse the pointer
    NSArray<NSValue *> *_masks;  // the parts whose picture is cut away
    VivaWebBridge *_bridge;
    NSTrackingArea *_tracking;
}
- (void)setBridge:(VivaWebBridge *)bridge;
- (void)setCutRects:(NSArray<NSValue *> *)cuts masks:(NSArray<NSValue *> *)masks;
@end

@implementation VivaWebView

- (void)dealloc {
    [_cuts release];
    [_masks release];
    [_bridge release];
    [_tracking release];
    [super dealloc];
}

- (void)setBridge:(VivaWebBridge *)bridge {
    [bridge retain];
    [_bridge release];
    _bridge = bridge;
}

- (NSView *)hitTest:(NSPoint)point {
    // The point is in the superview's coordinates.
    for (NSValue *value in _cuts) {
        if (NSPointInRect(point, value.rectValue)) {
            return nil;
        }
    }
    return [super hitTest:point];
}

- (void)applyMask {
    if (self.layer == nil) {
        return;
    }
    if (_masks.count == 0) {
        self.layer.mask = nil;
        return;
    }
    CAShapeLayer *mask = [CAShapeLayer layer];
    mask.frame = self.layer.bounds;
    CGMutablePathRef path = CGPathCreateMutable();
    CGPathAddRect(path, NULL, self.bounds);
    for (NSValue *value in _masks) {
        NSRect cut = [self convertRect:value.rectValue fromView:self.superview];
        CGPathAddRect(path, NULL, cut);
    }
    mask.path = path;
    mask.fillRule = kCAFillRuleEvenOdd;
    CGPathRelease(path);
    self.layer.mask = mask;
}

- (void)setCutRects:(NSArray<NSValue *> *)cuts masks:(NSArray<NSValue *> *)masks {
    if ([cuts isEqualToArray:_cuts] && [masks isEqualToArray:_masks]) {
        return;
    }
    [cuts retain];
    [_cuts release];
    _cuts = cuts;
    [masks retain];
    [_masks release];
    _masks = masks;
    [self applyMask];
}

- (void)setFrame:(NSRect)frame {
    [super setFrame:frame];
    [self applyMask];
}

- (void)updateTrackingAreas {
    [super updateTrackingAreas];
    if (_tracking != nil) {
        [self removeTrackingArea:_tracking];
        [_tracking release];
    }
    _tracking = [[NSTrackingArea alloc] initWithRect:self.bounds
                                             options:NSTrackingMouseEnteredAndExited | NSTrackingActiveInActiveApp | NSTrackingInVisibleRect
                                               owner:self
                                            userInfo:nil];
    [self addTrackingArea:_tracking];
}

// The pointer entering the page leaves Viva's drawing, which would otherwise keep the control it
// was over hovered.
- (void)mouseEntered:(NSEvent *)event {
    [self.superview mouseExited:event];
}

@end

void *viva_web_open(void *view, uint32_t id, uintptr_t handle, const char *url, const char *html,
                    const char *base, const char *script, const char *scheme, char **error) {
    __block VivaWebView *web = nil;
    viva_web_main(^{
        NSView *host = (NSView *)view;
        if (host == nil) {
            return;
        }
        VivaWebBridge *bridge = [[VivaWebBridge alloc] init];
        bridge->_handle = handle;
        bridge->_id = id;
        WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];
        WKUserContentController *controller = configuration.userContentController;
        [controller addScriptMessageHandler:bridge name:@"viva"];
        WKUserScript *bridgeScript = [[WKUserScript alloc] initWithSource:[NSString stringWithUTF8String:script]
                                                            injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                                         forMainFrameOnly:NO];
        [controller addUserScript:bridgeScript];
        [bridgeScript release];
        BOOL failed = NO;
        if (scheme != NULL && scheme[0] != 0) {
            @try {
                [configuration setURLSchemeHandler:bridge forURLScheme:[NSString stringWithUTF8String:scheme]];
            } @catch (NSException *exception) {
                failed = YES;
                if (error != NULL) {
                    *error = strdup(exception.reason.UTF8String);
                }
            }
        }
        if (!failed) {
            web = [[VivaWebView alloc] initWithFrame:NSMakeRect(0, 0, 1, 1) configuration:configuration];
            [web setBridge:bridge];
            web.navigationDelegate = bridge;
            web.autoresizingMask = NSViewMinYMargin;
            web.hidden = YES;
            [host addSubview:web positioned:NSWindowAbove relativeTo:nil];
            [web release]; // the view hierarchy owns it
        } else {
            [controller removeScriptMessageHandlerForName:@"viva"];
        }
        [configuration release];
        [bridge release];
        if (web == nil) {
            return;
        }
        if (html != NULL && html[0] != 0) {
            NSURL *baseURL = base != NULL && base[0] != 0 ? [NSURL URLWithString:[NSString stringWithUTF8String:base]] : nil;
            [web loadHTMLString:[NSString stringWithUTF8String:html] baseURL:baseURL];
        } else if (url != NULL && url[0] != 0) {
            NSURL *address = [NSURL URLWithString:[NSString stringWithUTF8String:url]];
            if (address != nil) {
                [web loadRequest:[NSURLRequest requestWithURL:address]];
            }
        }
    });
    return web;
}

void viva_web_load(void *web, const char *url) {
    viva_web_main(^{
        NSURL *address = [NSURL URLWithString:[NSString stringWithUTF8String:url]];
        if (address != nil) {
            [(VivaWebView *)web loadRequest:[NSURLRequest requestWithURL:address]];
        }
    });
}

void viva_web_load_html(void *web, const char *html, const char *base) {
    viva_web_main(^{
        NSURL *baseURL = base != NULL && base[0] != 0 ? [NSURL URLWithString:[NSString stringWithUTF8String:base]] : nil;
        [(VivaWebView *)web loadHTMLString:[NSString stringWithUTF8String:html] baseURL:baseURL];
    });
}

void viva_web_reload(void *web) {
    viva_web_main(^{ [(VivaWebView *)web reload]; });
}

void viva_web_back(void *web) {
    viva_web_main(^{ [(VivaWebView *)web goBack]; });
}

void viva_web_forward(void *web) {
    viva_web_main(^{ [(VivaWebView *)web goForward]; });
}

void viva_web_eval(void *web, const char *script) {
    viva_web_main(^{
        [(VivaWebView *)web evaluateJavaScript:[NSString stringWithUTF8String:script] completionHandler:nil];
    });
}

void viva_web_focus(void *web) {
    viva_web_main(^{
        VivaWebView *view = (VivaWebView *)web;
        if (!view.hidden) {
            [view.window makeFirstResponder:view];
        }
    });
}

void viva_web_close(void *web) {
    viva_web_main(^{
        VivaWebView *view = (VivaWebView *)web;
        [view.configuration.userContentController removeScriptMessageHandlerForName:@"viva"];
        view.navigationDelegate = nil;
        [view removeFromSuperview];
    });
}

void viva_web_place(void *web, int shown, double x, double y, double width, double height,
                    const double *cuts, size_t visibleCount, size_t count) {
    viva_web_main(^{
        VivaWebView *view = (VivaWebView *)web;
        NSView *host = view.superview;
        if (host == nil) {
            return;
        }
        CGFloat top = host.bounds.size.height;
        NSMutableArray<NSValue *> *rects = [NSMutableArray arrayWithCapacity:count];
        NSMutableArray<NSValue *> *masks = [NSMutableArray arrayWithCapacity:visibleCount];
        for (size_t index = 0; index < count; index++) {
            const double *cut = cuts + index * 4;
            NSValue *rect = [NSValue valueWithRect:NSMakeRect(cut[0], top - cut[1] - cut[3], cut[2], cut[3])];
            [rects addObject:rect];
            if (index < visibleCount) {
                [masks addObject:rect];
            }
        }
        [view setCutRects:rects masks:masks];
        if (!shown) {
            view.hidden = YES;
            return;
        }
        NSRect frame = NSMakeRect(x, top - y - height, width, height);
        if (!NSEqualRects(frame, view.frame)) {
            view.frame = frame;
        }
        view.hidden = NO;
    });
}

void viva_web_ready_later(uintptr_t handle) {
    dispatch_async(dispatch_get_main_queue(), ^{
        vivaWebReady(handle);
    });
}
