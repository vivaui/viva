//go:build ios

#ifndef VIVA_IOS_HOST_H
#define VIVA_IOS_HOST_H

/* Runs UIApplicationMain with Viva's application delegate. The Go side starts
 * the application from vivaIOSLaunched, once UIKit has launched it, and this
 * returns only if UIKit gives up. */
int viva_ios_main(void);

/* Ends the process, as an application that cannot go on must: iOS has no way to
 * quit gracefully. */
void viva_ios_terminate(int status);

/* Nonzero when the program runs in the iOS simulator, which takes another build
 * of the Metal library than a device. */
int viva_ios_is_simulator(void);

#endif
