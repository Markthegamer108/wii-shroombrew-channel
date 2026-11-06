#ifndef SHROOMBREW_DOL_H
#define SHROOMBREW_DOL_H

#include <gctypes.h>

#define DOL_TEXT 7
#define DOL_DATA 11

/* The 0x100-byte header of a DOL executable (all fields big-endian). */
typedef struct {
	u32 textOffset[DOL_TEXT];
	u32 dataOffset[DOL_DATA];
	u32 textAddr[DOL_TEXT];
	u32 dataAddr[DOL_DATA];
	u32 textSize[DOL_TEXT];
	u32 dataSize[DOL_DATA];
	u32 bssAddr;
	u32 bssSize;
	u32 entry;
	u32 pad[7];
} dol_header;

/* Where the position-independent stub runs from. Programs load low in MEM1, so
 * this sits near the top, clear of everything a normal program touches. */
#define STUB_ADDR 0x81780000u

int dol_validate(const dol_header *h, u32 fileSize, u32 reservedLo, u32 reservedHi);
void dol_set_argv(const char *path);
void dol_launch(const dol_header *h) __attribute__((noreturn));

#endif
