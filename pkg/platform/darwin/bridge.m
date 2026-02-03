// bridge.m - Objective-C implementation for cbterm platform layer

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "bridge.h"
#include "_cgo_export.h"

// Forward declarations
@interface CBTermAppDelegate : NSObject <NSApplicationDelegate>
@property (nonatomic, assign) BOOL debug;
@end

// URI Scheme Handler - calls back to Go for content
@interface CBTermSchemeHandler : NSObject <WKURLSchemeHandler>
@property (nonatomic, copy) NSString *scheme;
@end

// Global scheme registry
static NSMutableArray<NSString *> *gRegisteredSchemes = nil;
static CBTermSchemeHandler *gSchemeHandler = nil;

@interface CBTermWindowController : NSObject <NSWindowDelegate>
@property (nonatomic, strong) NSWindow *window;
@property (nonatomic, strong) WKWebView *webView;
@property (nonatomic, assign) void *closeCallback;
@end

@interface CBTermMessageHandler : NSObject <WKScriptMessageHandler>
@property (nonatomic, assign) void *callback;
@end

// Global state
static CBTermAppDelegate *gAppDelegate = nil;
static BOOL gDebug = NO;

#pragma mark - App Delegate

@implementation CBTermAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    // Make the app a proper foreground app
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    return YES;
}

@end

#pragma mark - Application Lifecycle

void cbterm_app_init(const char* name, bool debug) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        
        gAppDelegate = [[CBTermAppDelegate alloc] init];
        gAppDelegate.debug = debug;
        gDebug = debug;
        [NSApp setDelegate:gAppDelegate];
        
        // Set app name in menu bar
        NSString *appName = [NSString stringWithUTF8String:name];
        [[NSProcessInfo processInfo] setValue:appName forKey:@"processName"];
    }
}

void cbterm_app_run(void) {
    @autoreleasepool {
        [NSApp run];
    }
}

void cbterm_app_quit(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}

void cbterm_app_invoke(void* callback) {
    dispatch_async(dispatch_get_main_queue(), ^{
        goInvokeCallback(callback);
    });
}

#pragma mark - Window Controller

@implementation CBTermWindowController

- (instancetype)initWithTitle:(NSString *)title 
                        width:(int)width 
                       height:(int)height
                            x:(int)x
                            y:(int)y
                    resizable:(BOOL)resizable
                    frameless:(BOOL)frameless {
    self = [super init];
    if (self) {
        NSWindowStyleMask style = NSWindowStyleMaskTitled | 
                                  NSWindowStyleMaskClosable | 
                                  NSWindowStyleMaskMiniaturizable;
        if (resizable) {
            style |= NSWindowStyleMaskResizable;
        }
        if (frameless) {
            style = NSWindowStyleMaskBorderless | NSWindowStyleMaskResizable;
        }
        
        NSRect frame = NSMakeRect(x, y, width, height);
        self.window = [[NSWindow alloc] initWithContentRect:frame
                                                  styleMask:style
                                                    backing:NSBackingStoreBuffered
                                                      defer:NO];
        [self.window setTitle:title];
        [self.window setDelegate:self];
        
        // Create WebView configuration
        WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
        WKUserContentController *userContent = [[WKUserContentController alloc] init];
        config.userContentController = userContent;
        
        // Register custom URI scheme handlers
        if (gRegisteredSchemes && gSchemeHandler) {
            for (NSString *scheme in gRegisteredSchemes) {
                [config setURLSchemeHandler:gSchemeHandler forURLScheme:scheme];
            }
        }
        
        // Enable developer tools in debug mode
        if (gDebug) {
            [config.preferences setValue:@YES forKey:@"developerExtrasEnabled"];
        }
        
        // Create WebView
        self.webView = [[WKWebView alloc] initWithFrame:self.window.contentView.bounds
                                          configuration:config];
        self.webView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        
        // Allow file URLs if needed
        if (@available(macOS 10.11, *)) {
            [self.webView.configuration.preferences setValue:@YES 
                                                      forKey:@"allowFileAccessFromFileURLs"];
        }
        
        [self.window.contentView addSubview:self.webView];
        
        // Center if no position specified
        if (x == 0 && y == 0) {
            [self.window center];
        }
    }
    return self;
}

- (BOOL)windowShouldClose:(NSWindow *)sender {
    if (self.closeCallback) {
        return goWindowCloseCallback(self.closeCallback);
    }
    return YES;
}

@end

#pragma mark - Window Functions

CBTermWindow cbterm_window_new(const char* title, int width, int height, 
                                int x, int y, bool resizable, bool frameless) {
    __block CBTermWindowController *controller = nil;
    
    dispatch_block_t block = ^{
        controller = [[CBTermWindowController alloc] 
                      initWithTitle:[NSString stringWithUTF8String:title]
                              width:width
                             height:height
                                  x:x
                                  y:y
                          resizable:resizable
                          frameless:frameless];
    };
    
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
    
    return (__bridge_retained void *)controller;
}

void cbterm_window_show(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window makeKeyAndOrderFront:nil];
    });
}

void cbterm_window_hide(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window orderOut:nil];
    });
}

void cbterm_window_close(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window close];
    });
}

void cbterm_window_focus(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
    });
}

void cbterm_window_center(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window center];
    });
}

void cbterm_window_set_title(CBTermWindow w, const char* title) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    NSString *t = [NSString stringWithUTF8String:title];
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window setTitle:t];
    });
}

void cbterm_window_set_size(CBTermWindow w, int width, int height) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSRect frame = controller.window.frame;
        frame.size = NSMakeSize(width, height);
        [controller.window setFrame:frame display:YES animate:NO];
    });
}

void cbterm_window_set_position(CBTermWindow w, int x, int y) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window setFrameOrigin:NSMakePoint(x, y)];
    });
}

void cbterm_window_set_min_size(CBTermWindow w, int width, int height) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window setMinSize:NSMakeSize(width, height)];
    });
}

void cbterm_window_set_max_size(CBTermWindow w, int width, int height) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    dispatch_async(dispatch_get_main_queue(), ^{
        [controller.window setMaxSize:NSMakeSize(width, height)];
    });
}

void* cbterm_window_get_webview(CBTermWindow w) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    return (__bridge void *)controller.webView;
}

void cbterm_window_set_close_callback(CBTermWindow w, void* callback) {
    CBTermWindowController *controller = (__bridge CBTermWindowController *)w;
    controller.closeCallback = callback;
}

#pragma mark - Message Handler

@implementation CBTermMessageHandler

- (void)userContentController:(WKUserContentController *)userContentController
      didReceiveScriptMessage:(WKScriptMessage *)message {
    if (self.callback) {
        NSString *body = @"";
        if ([message.body isKindOfClass:[NSString class]]) {
            body = message.body;
        } else {
            NSData *data = [NSJSONSerialization dataWithJSONObject:message.body 
                                                           options:0 
                                                             error:nil];
            if (data) {
                body = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
            }
        }
        goMessageCallback(self.callback, (char *)[body UTF8String]);
    }
}

@end

#pragma mark - WebView Functions

void cbterm_webview_load_url(CBTermWebView wv, const char* url) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    NSString *urlString = [NSString stringWithUTF8String:url];
    dispatch_async(dispatch_get_main_queue(), ^{
        if ([urlString hasPrefix:@"file://"]) {
            NSString *path = [urlString substringFromIndex:7];
            NSURL *fileURL = [NSURL fileURLWithPath:path];
            // Grant read access to the directory containing index.html (for assets/)
            NSURL *dirURL = [fileURL URLByDeletingLastPathComponent];
            [webView loadFileURL:fileURL allowingReadAccessToURL:dirURL];
        } else {
            NSURL *nsurl = [NSURL URLWithString:urlString];
            [webView loadRequest:[NSURLRequest requestWithURL:nsurl]];
        }
    });
}

void cbterm_webview_load_html(CBTermWebView wv, const char* html, const char* base_url) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    NSString *htmlString = [NSString stringWithUTF8String:html];
    NSString *baseString = base_url ? [NSString stringWithUTF8String:base_url] : nil;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSURL *baseURL = baseString ? [NSURL URLWithString:baseString] : nil;
        [webView loadHTMLString:htmlString baseURL:baseURL];
    });
}

const char* cbterm_webview_eval(CBTermWebView wv, const char* js) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    NSString *jsString = [NSString stringWithUTF8String:js];
    
    __block NSString *result = @"";
    __block BOOL done = NO;
    
    dispatch_async(dispatch_get_main_queue(), ^{
        [webView evaluateJavaScript:jsString completionHandler:^(id _Nullable res, NSError * _Nullable error) {
            if (error) {
                result = [NSString stringWithFormat:@"error: %@", error.localizedDescription];
            } else if (res) {
                if ([res isKindOfClass:[NSString class]]) {
                    result = res;
                } else {
                    NSData *data = [NSJSONSerialization dataWithJSONObject:res options:0 error:nil];
                    if (data) {
                        result = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
                    }
                }
            }
            done = YES;
        }];
    });
    
    // Wait for completion (with timeout)
    NSDate *timeout = [NSDate dateWithTimeIntervalSinceNow:10.0];
    while (!done && [[NSDate date] compare:timeout] == NSOrderedAscending) {
        [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.01]];
    }
    
    return strdup([result UTF8String]);
}

void cbterm_webview_eval_async(CBTermWebView wv, const char* js) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    NSString *jsString = [NSString stringWithUTF8String:js];
    
    dispatch_async(dispatch_get_main_queue(), ^{
        [webView evaluateJavaScript:jsString completionHandler:nil];
    });
}

void cbterm_webview_free_string(const char* str) {
    free((void *)str);
}

void cbterm_webview_bind(CBTermWebView wv, const char* name, void* callback) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    NSString *funcName = [NSString stringWithUTF8String:name];
    
    dispatch_async(dispatch_get_main_queue(), ^{
        // Inject JavaScript that calls back to native
        NSString *script = [NSString stringWithFormat:@
            "window.%@ = function() {"
            "  var args = Array.prototype.slice.call(arguments);"
            "  window.webkit.messageHandlers._cbterm_bind_%@.postMessage(JSON.stringify(args));"
            "};", funcName, funcName];
        
        WKUserScript *userScript = [[WKUserScript alloc] 
                                    initWithSource:script
                                     injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                                  forMainFrameOnly:YES];
        [webView.configuration.userContentController addUserScript:userScript];
        
        // Add message handler for this binding
        CBTermMessageHandler *handler = [[CBTermMessageHandler alloc] init];
        handler.callback = callback;
        NSString *handlerName = [NSString stringWithFormat:@"_cbterm_bind_%@", funcName];
        [webView.configuration.userContentController addScriptMessageHandler:handler name:handlerName];
    });
}

void cbterm_webview_set_message_handler(CBTermWebView wv, void* callback) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    
    dispatch_async(dispatch_get_main_queue(), ^{
        CBTermMessageHandler *handler = [[CBTermMessageHandler alloc] init];
        handler.callback = callback;
        [webView.configuration.userContentController addScriptMessageHandler:handler name:@"cbterm"];
    });
}

void cbterm_webview_set_navigation_handler(CBTermWebView wv, void* callback) {
    // TODO: Implement navigation delegate
}

void cbterm_webview_reload(CBTermWebView wv) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    dispatch_async(dispatch_get_main_queue(), ^{
        [webView reload];
    });
}

void cbterm_webview_set_devtools(CBTermWebView wv, bool enabled) {
    WKWebView *webView = (__bridge WKWebView *)wv;
    dispatch_async(dispatch_get_main_queue(), ^{
        [webView.configuration.preferences setValue:@(enabled) forKey:@"developerExtrasEnabled"];
    });
}

#pragma mark - URI Scheme Handler

@implementation CBTermSchemeHandler

- (void)webView:(WKWebView *)webView startURLSchemeTask:(id<WKURLSchemeTask>)urlSchemeTask {
    NSURL *url = urlSchemeTask.request.URL;
    NSString *urlString = [url absoluteString];
    
    char *data = NULL;
    int length = 0;
    char *mimeType = NULL;
    
    // Call Go to get the resource
    goSchemeCallback((char *)[urlString UTF8String], &data, &length, &mimeType);
    
    if (data && length > 0) {
        NSData *responseData = [NSData dataWithBytes:data length:length];
        NSString *mime = mimeType ? [NSString stringWithUTF8String:mimeType] : @"application/octet-stream";
        
        // Use NSHTTPURLResponse for proper fetch() compatibility
        NSDictionary *headers = @{
            @"Content-Type": mime,
            @"Content-Length": [NSString stringWithFormat:@"%d", length],
            @"Access-Control-Allow-Origin": @"*"
        };
        
        NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:url
                                                                  statusCode:200
                                                                 HTTPVersion:@"HTTP/1.1"
                                                                headerFields:headers];
        
        [urlSchemeTask didReceiveResponse:response];
        [urlSchemeTask didReceiveData:responseData];
        [urlSchemeTask didFinish];
        
        // Free memory allocated by Go
        free(data);
        if (mimeType) free(mimeType);
    } else {
        // Return 404 with proper HTTP response
        NSDictionary *headers = @{
            @"Content-Type": @"text/plain"
        };
        NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:url
                                                                  statusCode:404
                                                                 HTTPVersion:@"HTTP/1.1"
                                                                headerFields:headers];
        [urlSchemeTask didReceiveResponse:response];
        [urlSchemeTask didReceiveData:[@"Not Found" dataUsingEncoding:NSUTF8StringEncoding]];
        [urlSchemeTask didFinish];
    }
}

- (void)webView:(WKWebView *)webView stopURLSchemeTask:(id<WKURLSchemeTask>)urlSchemeTask {
    // Request cancelled - nothing to clean up in our simple case
}

@end

void cbterm_register_scheme(const char* scheme) {
    @autoreleasepool {
        if (!gRegisteredSchemes) {
            gRegisteredSchemes = [[NSMutableArray alloc] init];
            gSchemeHandler = [[CBTermSchemeHandler alloc] init];
        }
        
        NSString *schemeStr = [NSString stringWithUTF8String:scheme];
        [gRegisteredSchemes addObject:schemeStr];
    }
}

#pragma mark - Menu Functions

CBTermMenu cbterm_menu_new(void) {
    __block NSMenu *menu = nil;
    dispatch_block_t block = ^{
        menu = [[NSMenu alloc] init];
    };
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_sync(dispatch_get_main_queue(), block);
    }
    return (__bridge_retained void *)menu;
}

// Menu item target that holds the callback
@interface CBTermMenuTarget : NSObject
@property (nonatomic, assign) void *callback;
- (void)menuItemClicked:(id)sender;
@end

@implementation CBTermMenuTarget
- (void)menuItemClicked:(id)sender {
    if (self.callback) {
        goMenuCallback(self.callback);
    }
}
@end

static NSMutableArray *gMenuTargets = nil;

void cbterm_menu_add_item(CBTermMenu m, const char* label, const char* shortcut, 
                          bool enabled, bool checked, void* callback) {
    NSMenu *menu = (__bridge NSMenu *)m;
    NSString *labelStr = [NSString stringWithUTF8String:label];
    NSString *shortcutStr = shortcut ? [NSString stringWithUTF8String:shortcut] : @"";
    
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!gMenuTargets) {
            gMenuTargets = [[NSMutableArray alloc] init];
        }
        
        // Parse shortcut (e.g., "Cmd+Q" -> key="q", modifiers=Command)
        NSString *key = @"";
        NSEventModifierFlags modifiers = 0;
        if (shortcutStr.length > 0) {
            NSArray *parts = [shortcutStr componentsSeparatedByString:@"+"];
            for (NSString *part in parts) {
                NSString *p = [part lowercaseString];
                if ([p isEqualToString:@"cmd"] || [p isEqualToString:@"command"]) {
                    modifiers |= NSEventModifierFlagCommand;
                } else if ([p isEqualToString:@"ctrl"] || [p isEqualToString:@"control"]) {
                    modifiers |= NSEventModifierFlagControl;
                } else if ([p isEqualToString:@"alt"] || [p isEqualToString:@"option"]) {
                    modifiers |= NSEventModifierFlagOption;
                } else if ([p isEqualToString:@"shift"]) {
                    modifiers |= NSEventModifierFlagShift;
                } else {
                    key = p;
                }
            }
        }
        
        CBTermMenuTarget *target = [[CBTermMenuTarget alloc] init];
        target.callback = callback;
        [gMenuTargets addObject:target];
        
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:labelStr
                                                      action:@selector(menuItemClicked:)
                                               keyEquivalent:key];
        [item setTarget:target];
        [item setKeyEquivalentModifierMask:modifiers];
        [item setEnabled:enabled];
        [item setState:checked ? NSControlStateValueOn : NSControlStateValueOff];
        [menu addItem:item];
    });
}

void cbterm_menu_add_separator(CBTermMenu m) {
    NSMenu *menu = (__bridge NSMenu *)m;
    dispatch_async(dispatch_get_main_queue(), ^{
        [menu addItem:[NSMenuItem separatorItem]];
    });
}

void cbterm_menu_add_submenu(CBTermMenu m, const char* label, CBTermMenu submenu) {
    NSMenu *menu = (__bridge NSMenu *)m;
    NSMenu *sub = (__bridge NSMenu *)submenu;
    NSString *labelStr = [NSString stringWithUTF8String:label];
    
    dispatch_async(dispatch_get_main_queue(), ^{
        NSMenuItem *item = [[NSMenuItem alloc] initWithTitle:labelStr action:nil keyEquivalent:@""];
        [item setSubmenu:sub];
        [sub setTitle:labelStr];
        [menu addItem:item];
    });
}

void cbterm_menu_clear(CBTermMenu m) {
    NSMenu *menu = (__bridge NSMenu *)m;
    dispatch_async(dispatch_get_main_queue(), ^{
        [menu removeAllItems];
    });
}

void cbterm_menu_popup(CBTermMenu m, int x, int y) {
    NSMenu *menu = (__bridge NSMenu *)m;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSPoint point;
        if (x == -1 && y == -1) {
            point = [NSEvent mouseLocation];
        } else {
            point = NSMakePoint(x, y);
        }
        [menu popUpMenuPositioningItem:nil atLocation:point inView:nil];
    });
}

void cbterm_app_set_menu(CBTermMenu m) {
    NSMenu *menu = (__bridge NSMenu *)m;
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp setMainMenu:menu];
    });
}
