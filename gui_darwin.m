#import <Cocoa/Cocoa.h>

#include "gui_darwin.h"
#include "_cgo_export.h" // declares goOnMuteClicked() / goOnExitClicked()

static void applyMuteColor(NSButton *b, int enabled);
static void applyLangColor(NSButton *b, int english);

// ControlTarget owns the button actions and bridges clicks back into Go.
@interface ControlTarget : NSObject
@property (strong) NSButton *muteButton;
@property (strong) NSButton *langButton;
- (void)muteClicked:(id)sender;
- (void)langClicked:(id)sender;
- (void)exitClicked:(id)sender;
@end

@implementation ControlTarget
- (void)muteClicked:(id)sender {
    // goOnMuteClicked toggles dictation and returns the new enabled state.
    int enabled = goOnMuteClicked();
    [self.muteButton setTitle:(enabled ? @"Mute" : @"Muted")];
    applyMuteColor(self.muteButton, enabled);
}
- (void)langClicked:(id)sender {
    // goOnLangClicked toggles the language and returns 1 (English) / 0 (Spanish).
    int english = goOnLangClicked();
    [self.langButton setTitle:(english ? @"EN" : @"ES")];
    applyLangColor(self.langButton, english);
}
- (void)exitClicked:(id)sender {
    goOnExitClicked(); // graceful shutdown; never returns (os.Exit in Go)
}
@end

static ControlTarget *gTarget = nil;
static NSWindow *gWindow = nil;

// Mute button: red while listening (enabled), gray while muted.
static void applyMuteColor(NSButton *b, int enabled) {
    if (b)
        [b setBezelColor:(enabled ? [NSColor systemRedColor]
                                  : [NSColor systemGrayColor])];
}

// Language button: red in EN, light blue ("celeste") in ES.
static void applyLangColor(NSButton *b, int english) {
    if (b)
        [b setBezelColor:(english ? [NSColor systemRedColor]
                                  : [NSColor colorWithSRGBRed:0.35 green:0.78 blue:0.98 alpha:1.0])];
}

void gui_run(int langIsEnglish) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        // Accessory: window shows and receives clicks, but no Dock icon and
        // no menu bar — appropriate for a background daemon.
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        NSRect vf = [[NSScreen mainScreen] visibleFrame];
        CGFloat w = 300, h = 72, margin = 16;
        NSRect frame = NSMakeRect(vf.origin.x + vf.size.width - w - margin,
                                  vf.origin.y + vf.size.height - h - margin,
                                  w, h);

        // Titled but not closable/resizable: the only way to quit is Exit.
        gWindow = [[NSWindow alloc] initWithContentRect:frame
                                              styleMask:NSWindowStyleMaskTitled
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
        [gWindow setTitle:@"Simon"];
        [gWindow setLevel:NSFloatingWindowLevel];
        [gWindow setMovableByWindowBackground:YES];
        [gWindow setReleasedWhenClosed:NO];

        gTarget = [[ControlTarget alloc] init];

        NSButton *mute = [[NSButton alloc] initWithFrame:NSMakeRect(12, 18, 88, 32)];
        [mute setTitle:@"Mute"];
        [mute setBezelStyle:NSBezelStyleRounded];
        [mute setTarget:gTarget];
        [mute setAction:@selector(muteClicked:)];
        gTarget.muteButton = mute;
        applyMuteColor(mute, 1); // starts listening



        NSButton *lang = [[NSButton alloc] initWithFrame:NSMakeRect(106, 18, 88, 32)];
        [lang setTitle:(langIsEnglish ? @"EN" : @"ES")];
        [lang setBezelStyle:NSBezelStyleRounded];
        [lang setTarget:gTarget];
        [lang setAction:@selector(langClicked:)];
        gTarget.langButton = lang;
        applyLangColor(lang, langIsEnglish);

        NSButton *ex = [[NSButton alloc] initWithFrame:NSMakeRect(200, 18, 88, 32)];
        [ex setTitle:@"Exit"];
        [ex setBezelStyle:NSBezelStyleRounded];
        [ex setTarget:gTarget];
        [ex setAction:@selector(exitClicked:)];

        [[gWindow contentView] addSubview:mute];
        [[gWindow contentView] addSubview:lang];
        [[gWindow contentView] addSubview:ex];

        [gWindow makeKeyAndOrderFront:nil];
        [NSApp activateIgnoringOtherApps:YES];
        [NSApp run]; // blocks the main thread until os.Exit
    }
}

void gui_set_mute_label(int enabled) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gTarget && gTarget.muteButton) {
            [gTarget.muteButton setTitle:(enabled ? @"Mute" : @"Muted")];
            applyMuteColor(gTarget.muteButton, enabled);
        }
    });
}
