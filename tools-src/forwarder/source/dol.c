/*
 * Loading a DOL and handing control to it.
 *
 * The hard part: the program being started will overwrite the memory this
 * forwarder is running from (everything links at 0x80003100). So the final
 * copy-and-jump is done by a small stub that is copied to STUB_ADDR first.
 * The stub must therefore be position independent: no libc calls, no globals,
 * only register work and relative branches. tools-src/forwarder/check.ps1
 * disassembles it to prove that.
 */
#include <gccore.h>
#include <ogc/cache.h>
#include <ogc/machine/processor.h>
#include <string.h>

#include "dol.h"

/* ---- the stub: runs from STUB_ADDR, interrupts off -------------------------- */

typedef void (*stub_fn)(const dol_header *h, u32 base);

__attribute__((noinline, noreturn)) void sb_stub(const dol_header *h, u32 base) {
	volatile u8 *dst, *src;
	u32 i, n, k;

	for (i = 0; i < DOL_TEXT + DOL_DATA; i++) {
		u32 off, addr, size;
		if (i < DOL_TEXT) {
			off = h->textOffset[i]; addr = h->textAddr[i]; size = h->textSize[i];
		} else {
			off = h->dataOffset[i - DOL_TEXT]; addr = h->dataAddr[i - DOL_TEXT]; size = h->dataSize[i - DOL_TEXT];
		}
		if (!size || !addr) continue;
		src = (volatile u8 *)(base + off);
		dst = (volatile u8 *)addr;
		for (n = 0; n < size; n++) dst[n] = src[n];
		/* make the new code visible to the instruction fetcher */
		for (k = 0; k < size + 32; k += 32) {
			__asm__ volatile("dcbst 0,%0 ; sync ; icbi 0,%0" ::"r"((u32)dst + k));
		}
	}
	dst = (volatile u8 *)h->bssAddr;
	for (n = 0; n < h->bssSize; n++) dst[n] = 0;
	for (k = 0; k < h->bssSize + 32; k += 32) __asm__ volatile("dcbst 0,%0" ::"r"((u32)dst + k));

	/* libogc points __system_argv at a struct just past the "_arg" marker in
	 * the program's startup code, which a loader is expected to fill in. The
	 * classic Homebrew Channel block at 0x80001800 is filled in already; put
	 * the same struct where this program looks for it. */
	{
		volatile u32 *e = (volatile u32 *)h->entry;
		u32 k2;
		for (k2 = 0; k2 < 16; k2++) {
			if (e[k2] == 0x5f617267u) {
				volatile u32 *a = e + k2 + 1; /* magic, command line, length, argc, argv, endARGV */
				if (a[0] == 0 && a[1] == 0 && a[2] == 0 && a[3] == 0 && a[4] == 0 && a[5] == 0) {
					a[0] = 0x5f617267u;
					a[1] = 0x80001840u;
					a[2] = *(volatile u32 *)0x8000183cu;
					a[3] = 1;
					a[4] = 0x80001880u;
					a[5] = 0x80001884u;
					__asm__ volatile("dcbst 0,%0 ; sync" ::"r"(a));
					break;
				}
			}
		}
	}

	{
		u32 entry = h->entry;
		__asm__ volatile("sync ; isync");
		((void (*)(void))entry)();
	}
	for (;;) {}
}

/* Marks where the stub ends. Must stay directly after sb_stub (built with -fno-toplevel-reorder). */
__attribute__((noinline)) void sb_stub_end(void) { __asm__ volatile("nop"); }

/* ---- helpers ---------------------------------------------------------------- */

static u32 be(u32 v) { return v; } /* PowerPC is big-endian: DOL fields are native */

int dol_validate(const dol_header *h, u32 fileSize, u32 reservedLo, u32 reservedHi) {
	u32 i, lo = 0xffffffffu, hi = 0;
	int any = 0;
	for (i = 0; i < DOL_TEXT + DOL_DATA; i++) {
		u32 off, addr, size;
		if (i < DOL_TEXT) { off = h->textOffset[i]; addr = h->textAddr[i]; size = h->textSize[i]; }
		else { off = h->dataOffset[i - DOL_TEXT]; addr = h->dataAddr[i - DOL_TEXT]; size = h->dataSize[i - DOL_TEXT]; }
		if (!size) continue;
		any = 1;
		if (off < sizeof(dol_header) || off > fileSize || size > fileSize - off) return 0;
		/* sections must land in MEM1 or MEM2 and must not touch the stub */
		if (!((addr >= 0x80000000u && addr + size <= 0x81800000u) || (addr >= 0x90000000u && addr + size <= 0x94000000u))) return 0;
		if (addr + size < addr) return 0;
		if (addr < lo) lo = addr;
		if (addr + size > hi) hi = addr + size;
		if (addr < STUB_ADDR + 0x10000 && addr + size > STUB_ADDR) return 0;
		/* the program file itself is parked in this range; copying a section
		 * on top of it would corrupt the remaining contents */
		if (addr < reservedHi && addr + size > reservedLo) return 0;
	}
	if (!any) return 0;
	if (h->bssSize && !((h->bssAddr >= 0x80000000u && h->bssAddr + h->bssSize <= 0x81800000u) || (h->bssAddr >= 0x90000000u && h->bssAddr + h->bssSize <= 0x94000000u))) return 0;
	if (h->bssSize && h->bssAddr < reservedHi && h->bssAddr + h->bssSize > reservedLo) return 0;
	if (h->entry < 0x80000000u || h->entry >= 0x94000000u) return 0;
	(void)be;
	(void)lo;
	(void)hi;
	return 1;
}

/* Homebrew Channel convention: a record at 0x80001800 that tells the program
 * its own path, which most apps use to find their data files. The command line
 * and its vector live at fixed addresses just after the record. */
void dol_set_argv(const char *path) {
	volatile u32 *a = (volatile u32 *)0x80001800;
	char *str = (char *)0x80001840;
	char **vec = (char **)0x80001880;
	size_t len = strlen(path) + 1;
	u32 i;

	for (i = 0; i < 0x40 / 4; i++) a[i] = 0; /* clear the record area */
	memcpy(str, path, len);
	vec[0] = str;
	vec[1] = NULL;

	a[0] = 0x5f617267;           /* argvMagic */
	a[1] = (u32)str;             /* commandLine */
	a[2] = (u32)len;             /* length, also read by the stub */
	a[3] = 1;                    /* argc */
	a[4] = (u32)vec;             /* argv */
	a[5] = (u32)(vec + 1);       /* endARGV */
	*(volatile u32 *)0x8000183c = (u32)len;
	DCFlushRange((void *)0x80001800, 0x100);
}

void dol_launch(const dol_header *h) {
	u32 base = (u32)h;
	size_t stubLen = (size_t)((u8 *)sb_stub_end - (u8 *)sb_stub);
	u32 level;

	_CPU_ISR_Disable(level);
	(void)level;

	memcpy((void *)STUB_ADDR, (void *)sb_stub, stubLen);
	DCFlushRange((void *)STUB_ADDR, stubLen + 32);
	ICInvalidateRange((void *)STUB_ADDR, stubLen + 32);

	/* Jump to the relocated stub on a stack that lives next to it, because the
	 * stack we are on sits in memory the program is about to overwrite. */
	__asm__ volatile(
		"mr 3,%0 ; mr 4,%1 ; mr 12,%2 ;"
		"lis 1,0x8179 ; subi 1,1,16 ;"
		"mtctr 12 ; bctr"
		:: "r"(h), "r"(base), "r"(STUB_ADDR) : "r3", "r4", "r12", "memory");
	for (;;) {}
}
