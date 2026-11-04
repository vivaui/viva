#ifndef VIVA_DOCK_PROGRESS_DARWIN_H
#define VIVA_DOCK_PROGRESS_DARWIN_H

/* Draws a bar of the given length along the foot of the application's Dock
 * tile, over the application's own icon, where fraction is between 0 and 1.
 * A negative fraction, or one that is not a number, takes the bar away and
 * leaves the tile as the system draws it. Main thread only. */
void viva_application_set_progress(double fraction);

#endif
