#ifndef VIVA_FILEWATCH_DARWIN_H
#define VIVA_FILEWATCH_DARWIN_H

#include <stdint.h>

typedef struct viva_fsevents viva_fsevents;

// viva_fsevents_open starts an FSEvents stream on path; every event is reported
// through vivaFileEvent with handle. It returns NULL when the stream cannot start.
viva_fsevents *viva_fsevents_open(const char *path, uintptr_t handle);

// viva_fsevents_close stops the stream and returns only after no callback is
// running or will run.
void viva_fsevents_close(viva_fsevents *stream);

#endif
