#ifndef VIVA_FILE_ICON_DARWIN_H
#define VIVA_FILE_ICON_DARWIN_H

/* Draws the icon Finder shows for the file or folder at path into out, a
 * pixels x pixels buffer of premultiplied BGRA, top row first, which the
 * caller owns and which this function overwrites. A path that does not exist
 * asks for the icon of the type its extension names, since there is no file
 * for the system to look at. Returns 0, or -1 with *error_message set to a
 * string the caller frees with viva_system_free. Safe from any thread:
 * NSWorkspace documents iconForFile: that way, the type lookup only reads the
 * system's database, and the drawing goes to a private bitmap context. */
int viva_file_icon_bgra(const char *path, int pixels, unsigned char *out, char **error_message);

#endif
