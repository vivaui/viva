#import "../save_panel_darwin.m"
#include <stdio.h>
#include <string.h>

int main(int argc, const char *argv[]) {
    if (argc != 2) {
        return 2;
    }
    @autoreleasepool {
        [NSApplication sharedApplication].activationPolicy = NSApplicationActivationPolicyAccessory;
        NSURL *directory = [NSURL fileURLWithPath:[NSString stringWithUTF8String:argv[1]] isDirectory:YES];
        NSArray<NSString *> *names = @[@"draft.go", @"note.mh", @"draft", @"note.mh", @"aa.bb.go"];
        NSArray<NSArray<NSString *> *> *types = @[@[@"go"], @[@"mh"], @[@"go"], @[@"go", @"mh"], @[@"go"]];
        NSArray<NSString *> *expected = @[@"draft.go", @"note.mh", @"draft.go", @"note.mh", @"aa.bb.go"];
        for (NSUInteger index = 0; index < names.count; index++) {
            NSSavePanel *panel = [NSSavePanel savePanel];
            panel.directoryURL = directory;
            viva_configure_save_panel(panel, names[index], types[index]);
            [panel orderFront:nil];
            NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:2.0];
            while ((panel.URL == nil || [panel.URL.lastPathComponent isEqualToString:@"Untitled"]) &&
                   deadline.timeIntervalSinceNow > 0) {
                [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode
                                       beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.02]];
            }
            NSString *filename = panel.URL.lastPathComponent;
            if (![filename isEqualToString:expected[index]]) {
                fprintf(stderr, "%s: panel URL is %s, want %s\n", names[index].UTF8String,
                    filename.UTF8String, expected[index].UTF8String);
                [panel orderOut:nil];
                return 1;
            }
            [panel orderOut:nil];
        }
    }
    return 0;
}
