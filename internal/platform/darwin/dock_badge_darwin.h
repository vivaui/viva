//go:build !ios

#ifndef VIVA_DOCK_BADGE_DARWIN_H
#define VIVA_DOCK_BADGE_DARWIN_H

/* Sets the text of the red badge on the application's Dock tile, or removes the
 * badge for NULL. Main thread only. */
void viva_application_set_badge_label(const char *label);

#endif
