#ifndef SHROOMBREW_PAYLOAD_H
#define SHROOMBREW_PAYLOAD_H

#include <gctypes.h>

/* The channel carries the app as a ZIP in this content index of its own title. */
#define PAYLOAD_CONTENT_INDEX 2

/* Opens the package carried by this channel and returns its size in bytes, or 0
 * when this channel has none (for example when the forwarder is started as a plain
 * DOL during development). The package is read sequentially and never held in
 * memory as a whole, so its size is limited only by the space on the SD card. */
u32 payload_open(void);

/* Opens a package file on the SD card instead. Development aid. */
u32 payload_open_file(const char *path);

/* Unpacks every file under apps/ onto the SD card while reading the package once,
 * front to back. progress receives the bytes of the package read so far and its
 * total size. Returns the number of files written, or a negative code (see
 * payload_error_text). Nothing outside sd:/apps/ is ever written, and unsafe names
 * are refused rather than skipped. */
int payload_extract(void (*progress)(u32 done, u32 total));

void payload_close(void);

const char *payload_error_text(int code);

#endif
