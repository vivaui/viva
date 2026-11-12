#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <WebKit/WebKit.h>

#include <stdlib.h>
#include <string.h>

#include "web_darwin.h"

// What the page does reaches Go through these (web_darwin.go).
extern void vivaWebMessage(uintptr_t handle, uint32_t id, char *utf8);
extern void vivaWebLoad(uintptr_t handle, uint32_t id, int phase, char *url);
extern int vivaWebNavigate(uintptr_t handle, uint32_t id, char *url, int mainFrame, int userGesture);
extern int vivaWebNewWindow(uintptr_t handle, uint32_t id, char *url);
extern int vivaWebDownload(uintptr_t handle, uint32_t id, char *url);
extern char *vivaWebDownloadPath(char *folder, char *suggested, char *address);
extern int vivaWebServe(uintptr_t handle, uint32_t id, char *scheme, char *path, int *status,
                        char **headers, unsigned char **data, size_t *length);
extern int vivaWebServeAsync(uintptr_t handle, uint32_t id, char *scheme, char *path, uint64_t request);
extern void vivaWebReady(uintptr_t handle);

static void viva_web_main(void (^work)(void)) {
    if ([NSThread isMainThread]) {
        work();
    } else {
        dispatch_sync(dispatch_get_main_queue(), work);
    }
}

// VivaWebBridge is everything the page may call: its script messages, its navigation, requests to
// its custom scheme, and the downloads it starts.
@interface VivaWebBridge : NSObject <WKScriptMessageHandler, WKNavigationDelegate, WKURLSchemeHandler, WKDownloadDelegate> {
@public
    uintptr_t _handle;
    uint32_t _id;
    // The async requests still to be answered, by request id, and the reverse of the id by task, so
    // stopURLSchemeTask can find one. The dictionary retains the task: a task is never messaged
    // after it is removed from here, which is what keeps a late answer from touching a released one.
    NSMutableDictionary<NSNumber *, id<WKURLSchemeTask>> *_tasks;
    NSMutableDictionary<NSValue *, NSNumber *> *_taskIDs;
    NSMutableArray<WKDownload *> *_downloads; // kept until each finishes, since WKDownload keeps a weak delegate
}
- (void)answerRequest:(uint64_t)request status:(int)status headers:(char *)headers data:(unsigned char *)data length:(size_t)length;
- (void)cancelTasks;
@end

// The bridge that owns each outstanding request, and the request ids, both touched on the main thread
// only: start and stop are main-thread calls, and an answer from Go is marshaled there first.
static NSMutableDictionary<NSNumber *, VivaWebBridge *> *viva_web_answers;
static uint64_t viva_web_next_request;

@implementation VivaWebBridge

- (instancetype)init {
    self = [super init];
    if (self != nil) {
        _tasks = [[NSMutableDictionary alloc] init];
        _taskIDs = [[NSMutableDictionary alloc] init];
        _downloads = [[NSMutableArray alloc] init];
    }
    return self;
}

- (void)dealloc {
    [_tasks release];
    [_taskIDs release];
    [_downloads release];
    [super dealloc];
}

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

// The application answers before the request leaves the engine: a refused navigation fetches
// nothing and runs no script. A nil target frame is a new window, which createWebView decides;
// letting the policy through fetches nothing until that returns nil.
- (void)webView:(WKWebView *)web decidePolicyForNavigationAction:(WKNavigationAction *)action decisionHandler:(void (^)(WKNavigationActionPolicy))decisionHandler {
    (void)web;
    NSString *address = action.request.URL.absoluteString;
    char *url = (char *)(address != nil ? address.UTF8String : "");
    if (@available(macOS 11.3, *)) {
        if (action.shouldPerformDownload) {
            decisionHandler(vivaWebDownload(_handle, _id, url) ? WKNavigationActionPolicyDownload : WKNavigationActionPolicyCancel);
            return;
        }
    }
    if (action.targetFrame == nil) {
        decisionHandler(WKNavigationActionPolicyAllow);
        return;
    }
    // WKWebView has no public user-gesture flag. The navigation type is the closest it gets: a
    // click on a link and a form submission are gestures, while a script's location change is
    // Other. A form a script submits is therefore reported as a gesture on this engine.
    BOOL gesture = action.navigationType == WKNavigationTypeLinkActivated || action.navigationType == WKNavigationTypeFormSubmitted;
    decisionHandler(vivaWebNavigate(_handle, _id, url, action.targetFrame.isMainFrame ? 1 : 0, gesture ? 1 : 0)
                        ? WKNavigationActionPolicyAllow
                        : WKNavigationActionPolicyCancel);
}

// The page never opens a window of its own: an allowed window.open or target=_blank goes to the
// system's browser, and nil refuses it before anything is fetched.
- (WKWebView *)webView:(WKWebView *)web configuration:(WKWebViewConfiguration *)configuration forNavigationAction:(WKNavigationAction *)action windowFeatures:(WKWindowFeatures *)windowFeatures {
    (void)web;
    (void)configuration;
    (void)windowFeatures;
    NSString *address = action.request.URL.absoluteString;
    if (address != nil) {
        vivaWebNewWindow(_handle, _id, (char *)address.UTF8String);
    }
    return nil;
}

// A response WebKit cannot show, or one the server marks as an attachment, is a download the
// application decides before its body is written.
- (void)webView:(WKWebView *)web decidePolicyForNavigationResponse:(WKNavigationResponse *)response decisionHandler:(void (^)(WKNavigationResponsePolicy))decisionHandler {
    (void)web;
    NSURLResponse *nativeResponse = response.response;
    NSHTTPURLResponse *http = [nativeResponse isKindOfClass:[NSHTTPURLResponse class]] ? (NSHTTPURLResponse *)nativeResponse : nil;
    NSString *disposition = [http valueForHTTPHeaderField:@"Content-Disposition"];
    BOOL attachment = disposition != nil && [disposition.lowercaseString hasPrefix:@"attachment"];
    if (response.canShowMIMEType && !attachment) {
        decisionHandler(WKNavigationResponsePolicyAllow);
        return;
    }
    if (@available(macOS 11.3, *)) {
        NSString *address = nativeResponse.URL.absoluteString;
        char *url = (char *)(address != nil ? address.UTF8String : "");
        decisionHandler(vivaWebDownload(_handle, _id, url) ? WKNavigationResponsePolicyDownload : WKNavigationResponsePolicyCancel);
        return;
    }
    // WKDownload is 11.3; before it a download cannot be saved, and a page does not write files on
    // its own, so the response is refused instead.
    decisionHandler(WKNavigationResponsePolicyCancel);
}

// A download keeps its delegate alive from here to its last byte: the bridge holds it, because
// WKDownload's delegate is weak.
- (void)webView:(WKWebView *)web navigationResponse:(WKNavigationResponse *)response didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    (void)web;
    (void)response;
    download.delegate = self;
    [_downloads addObject:download];
}

- (void)webView:(WKWebView *)web navigationAction:(WKNavigationAction *)action didBecomeDownload:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    (void)web;
    (void)action;
    download.delegate = self;
    [_downloads addObject:download];
}

// The destination is a free name in the user's downloads folder, so a second download of the same
// name never replaces the first.
- (void)download:(WKDownload *)download decideDestinationUsingResponse:(NSURLResponse *)response suggestedFilename:(NSString *)suggestedFilename completionHandler:(void (^)(NSURL * _Nullable))completionHandler API_AVAILABLE(macos(11.3)) {
    NSString *folder = NSSearchPathForDirectoriesInDomains(NSDownloadsDirectory, NSUserDomainMask, YES).firstObject;
    NSString *address = response.URL.absoluteString;
    char *path = vivaWebDownloadPath((char *)(folder != nil ? folder.UTF8String : ""),
                                     (char *)(suggestedFilename != nil ? suggestedFilename.UTF8String : ""),
                                     (char *)(address != nil ? address.UTF8String : ""));
    if (path == NULL) {
        completionHandler(nil);
        return;
    }
    NSString *destination = [NSString stringWithUTF8String:path];
    free(path);
    completionHandler(destination != nil ? [NSURL fileURLWithPath:destination] : nil);
}

- (void)downloadDidFinish:(WKDownload *)download API_AVAILABLE(macos(11.3)) {
    [_downloads removeObject:download];
}

- (void)download:(WKDownload *)download didFailWithError:(NSError *)error API_AVAILABLE(macos(11.3)) {
    (void)error;
    [_downloads removeObject:download];
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

// finishTask answers task and lets it go. headers is the header block Go resolved: one "Name: value"
// line each.
- (void)finishTask:(id<WKURLSchemeTask>)task status:(int)status headers:(char *)headers data:(unsigned char *)data length:(size_t)length {
    NSMutableDictionary *fields = [NSMutableDictionary dictionary];
    if (headers != NULL) {
        NSString *block = [NSString stringWithUTF8String:headers];
        for (NSString *line in [block componentsSeparatedByString:@"\r\n"]) {
            NSRange colon = [line rangeOfString:@":"];
            if (colon.location == NSNotFound) {
                continue;
            }
            NSString *name = [[line substringToIndex:colon.location] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
            NSString *value = [[line substringFromIndex:colon.location + 1] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
            if (name.length > 0) {
                [fields setObject:value forKey:name];
            }
        }
    }
    NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:task.request.URL
                                                              statusCode:status
                                                             HTTPVersion:@"HTTP/1.1"
                                                            headerFields:fields];
    [task didReceiveResponse:response];
    [response release];
    if (data != NULL && length > 0) {
        [task didReceiveData:[NSData dataWithBytes:data length:length]];
    }
    [task didFinish];
}

// answerRequest is the answer to an async request, on the main queue. A request that was stopped is
// no longer in the dictionaries, and its task is not touched.
- (void)answerRequest:(uint64_t)request status:(int)status headers:(char *)headers data:(unsigned char *)data length:(size_t)length {
    NSNumber *key = [NSNumber numberWithUnsignedLongLong:request];
    // The task is let go of only after it is answered: removing the dictionary's reference before
    // messaging it could release the task the last time anyone holds it.
    id<WKURLSchemeTask> task = [[_tasks objectForKey:key] retain];
    if (task == nil) {
        return;
    }
    [_taskIDs removeObjectForKey:[NSValue valueWithNonretainedObject:task]];
    [_tasks removeObjectForKey:key];
    [viva_web_answers removeObjectForKey:key];
    [self finishTask:task status:status headers:headers data:data length:length];
    [task release];
}

// cancelTasks lets go of the requests of a page that is closing. WebKit may not stop each one, and the
// registry would otherwise keep the bridge alive.
- (void)cancelTasks {
    for (NSNumber *key in _tasks) {
        id<WKURLSchemeTask> task = [_tasks objectForKey:key];
        [_taskIDs removeObjectForKey:[NSValue valueWithNonretainedObject:task]];
        [viva_web_answers removeObjectForKey:key];
    }
    [_tasks removeAllObjects];
}

- (void)webView:(WKWebView *)web startURLSchemeTask:(id<WKURLSchemeTask>)task {
    (void)web;
    NSURL *url = task.request.URL;
    NSString *path = url.path != nil ? url.path : @"";
    if ([path hasPrefix:@"/"]) {
        path = [path substringFromIndex:1];
    }
    char *scheme = (char *)url.scheme.UTF8String;
    char *chars = (char *)path.UTF8String;
    if (viva_web_answers == nil) {
        viva_web_answers = [[NSMutableDictionary alloc] init];
    }
    viva_web_next_request++;
    uint64_t request = viva_web_next_request;
    NSNumber *key = [NSNumber numberWithUnsignedLongLong:request];
    // Register the task before the call: ServeAsync may close its own page, and Close cancels the
    // tasks registered at that moment. The answer always hops to the main queue, so it is delivered
    // after this method returns even when ServeAsync answers synchronously.
    [_tasks setObject:task forKey:key];
    [_taskIDs setObject:key forKey:[NSValue valueWithNonretainedObject:task]];
    [viva_web_answers setObject:self forKey:key];
    if (vivaWebServeAsync(_handle, _id, scheme, chars, request)) {
        return;
    }
    // The scheme has no asynchronous form (or this page is not its): withdraw the registration before
    // the synchronous answer, which finishes the task here and now.
    [_taskIDs removeObjectForKey:[NSValue valueWithNonretainedObject:task]];
    [_tasks removeObjectForKey:key];
    [viva_web_answers removeObjectForKey:key];
    int status = 404;
    char *headers = NULL;
    unsigned char *data = NULL;
    size_t length = 0;
    if (!vivaWebServe(_handle, _id, scheme, chars, &status, &headers, &data, &length)) {
        status = 404;
        headers = strdup("Content-Type: text/plain\r\nContent-Length: 0\r\nAccess-Control-Allow-Origin: *");
    }
    [self finishTask:task status:status headers:headers data:data length:length];
    free(headers);
    free(data);
}

- (void)webView:(WKWebView *)web stopURLSchemeTask:(id<WKURLSchemeTask>)task {
    (void)web;
    NSValue *key = [NSValue valueWithNonretainedObject:task];
    NSNumber *request = [_taskIDs objectForKey:key];
    if (request == nil) {
        return;
    }
    [_taskIDs removeObjectForKey:key];
    [_tasks removeObjectForKey:request]; // releases the task: a later answer finds no entry
    [viva_web_answers removeObjectForKey:request];
}

@end

// viva_web_scheme_answer takes the answer from Go, on any thread, and touches the task on the main
// queue only, after the page may have closed. The buffers are freed here either way.
void viva_web_scheme_answer(uint64_t request, int status, char *headers, unsigned char *data, size_t length) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSNumber *key = [NSNumber numberWithUnsignedLongLong:request];
        VivaWebBridge *bridge = [viva_web_answers objectForKey:key];
        if (bridge != nil) {
            [bridge answerRequest:request status:status headers:headers data:data length:length];
        }
        free(headers);
        free(data);
    });
}

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
- (void)cancelSchemeTasks;
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

- (void)cancelSchemeTasks {
    [_bridge cancelTasks];
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
                    const char *base, const char *script, int mainFrameOnly, const char *scheme, char **error) {
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
                                                         forMainFrameOnly:mainFrameOnly ? YES : NO];
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
        [view cancelSchemeTasks];
        [view.configuration.userContentController removeScriptMessageHandlerForName:@"viva"];
        view.navigationDelegate = nil;
        [view removeFromSuperview];
    });
}

void viva_web_cancel_scheme_tasks(void *web) {
    viva_web_main(^{
        [(VivaWebView *)web cancelSchemeTasks];
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
