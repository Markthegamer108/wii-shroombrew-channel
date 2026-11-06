/*
 * Shroombrew channel.
 *
 * One small program, built once, that the server turns into a Wii Menu channel
 * for each homebrew app by patching the CONFIG block below and attaching the
 * app's files as a package inside the channel (see internal/channelpkg).
 *
 * On launch it
 *   1. makes sure the app is on the SD card: if the files are missing, or an
 *      older version is there, it unpacks the package it carries;
 *   2. loads the app's boot.dol from the SD card and runs it.
 *
 * Started as a plain DOL (development) there is no package, and it simply runs
 * what is already on the card.
 *
 * A short log is kept at sd:/shroombrew-channel.log so a console that shows an
 * error can be diagnosed without special tools.
 *
 * Original work for Shroombrew. Built with devkitPPC + libogc + libfat + zlib.
 */
#include <dirent.h>
#include <gccore.h>
#include <fat.h>
#include <malloc.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#include "dol.h"
#include "payload.h"

#define CFG_MAGIC "SHROOMBREW-FWD-CFG\0"
#define CFG_AREA 192

/* Layout after the magic: "<boot.dol path>\0<version>\0". The synthesizer must find
 * exactly one copy of the magic in the DOL to patch. */
__attribute__((used, aligned(4))) static volatile char config[sizeof(CFG_MAGIC) + CFG_AREA] =
	CFG_MAGIC "sd:/apps/shroombrew-unconfigured/boot.dol\0unconfigured";

static GXRModeObj *rmode = NULL;
static int video_ready = 0;
static FILE *g_log = NULL;

static void video_start(void) {
	if (video_ready) return;
	video_ready = 1;
	VIDEO_Init();
	rmode = VIDEO_GetPreferredMode(NULL);
	void *xfb = MEM_K0_TO_K1(SYS_AllocateFramebuffer(rmode));
	console_init(xfb, 20, 20, rmode->fbWidth, rmode->xfbHeight, rmode->fbWidth * VI_DISPLAY_PIX_SZ);
	VIDEO_Configure(rmode);
	VIDEO_SetNextFramebuffer(xfb);
	VIDEO_SetBlack(false);
	VIDEO_Flush();
	VIDEO_WaitVSync();
	if (rmode->viTVMode & VI_NON_INTERLACE) VIDEO_WaitVSync();
}

static void log_open(void) {
	g_log = fopen("sd:/shroombrew-channel.log", "wb");
}

static void log_add(const char *fmt, ...) {
	va_list ap;
	if (!g_log) return;
	va_start(ap, fmt);
	vfprintf(g_log, fmt, ap);
	va_end(ap);
	fputc('\n', g_log);
	fflush(g_log);
}

static void log_close(void) {
	if (g_log) {
		fclose(g_log);
		g_log = NULL;
	}
}

static void fail(const char *what, const char *detail) {
	log_add("FAILED: %s", what);
	log_add("detail: %s", detail);
	log_close();
	video_start();
	printf("\n\n  Shroombrew\n\n  %s\n\n  %s\n\n", what, detail);
	printf("  Download the app again from the Shroombrew shop,\n  then start this channel again.\n\n");
	printf("  Returning to the Wii Menu...\n");
	sleep(7);
	SYS_ResetSystem(SYS_RETURNTOMENU, 0, 0);
	exit(1);
}

static void show_progress(u32 done, u32 total) {
	printf("\r  Installing... %u / %u MB   ", (unsigned)(done >> 20), (unsigned)(total >> 20));
}

static int file_exists(const char *path) {
	FILE *f = fopen(path, "rb");
	if (!f) return 0;
	fclose(f);
	return 1;
}

/* "<dir of boot.dol>/.shroombrew-version" */
static void marker_path(const char *bootPath, char *out, size_t cap) {
	const char *slash = strrchr(bootPath, '/');
	size_t dir = slash ? (size_t)(slash - bootPath) : 0;
	if (dir + 24 > cap) dir = cap - 24;
	memcpy(out, bootPath, dir);
	strcpy(out + dir, "/.shroombrew-version");
}

static int marker_matches(const char *marker, const char *version) {
	char buf[64];
	size_t n;
	FILE *f = fopen(marker, "rb");
	if (!f) return 0;
	n = fread(buf, 1, sizeof(buf) - 1, f);
	fclose(f);
	buf[n] = 0;
	return strcmp(buf, version) == 0;
}

/* Development aid: what the channel can actually see on the card. */
static void log_card_root(void) {
	DIR *d = opendir("sd:/");
	struct dirent *e;
	if (!d) {
		log_add("could not list the SD card");
		return;
	}
	while ((e = readdir(d)) != NULL) log_add("sd: %s", e->d_name);
	closedir(d);
}

static void log_file_size(const char *p) {
	FILE *f = fopen(p, "rb");
	long n;
	if (!f) {
		log_add("cannot open %s", p);
		return;
	}
	fseek(f, 0, SEEK_END);
	n = ftell(f);
	fclose(f);
	log_add("%s: %ld bytes", p, n);
}

int main(void) {
	const char *cfg = (const char *)config + sizeof(CFG_MAGIC) - 1;
	char path[CFG_AREA + 1], version[64], marker[CFG_AREA + 32];
	const char *v;
	long size;
	FILE *f;
	u32 base, top;

	strncpy(path, cfg, CFG_AREA);
	path[CFG_AREA] = 0;
	v = cfg + strlen(path) + 1;
	strncpy(version, (v < cfg + CFG_AREA) ? v : "", sizeof(version) - 1);
	version[sizeof(version) - 1] = 0;

	if (!fatInitDefault()) fail("Could not read the SD card.", path);
	log_open();
	log_add("Shroombrew channel %s", version);
	log_add("app: %s", path);

	marker_path(path, marker, sizeof(marker));
	{
		int present = file_exists(path);
		int current = marker_matches(marker, version);
		log_add("app present: %s, version marker: %s", present ? "yes" : "no", current ? "match" : "stale");
		if (!present || !current) {
			u32 pkgSize = payload_open();
			if (!pkgSize) {
				log_card_root();
				log_file_size("sd:/shroombrew-payload.zip");
				pkgSize = payload_open_file("sd:/shroombrew-payload.zip");
				if (pkgSize) log_add("package: %u bytes from sd:/shroombrew-payload.zip (development)", (unsigned)pkgSize);
			} else {
				log_add("package: %u bytes from the channel", (unsigned)pkgSize);
			}
			if (pkgSize) {
				int rc;
				video_start();
				printf("\n\n  Shroombrew\n\n  Setting up the app on your SD card.\n  Please do not remove the card.\n\n");
				rc = payload_extract(show_progress);
				payload_close();
				if (rc < 0) fail(payload_error_text(rc), path);
				log_add("extracted %d files", rc);
				f = fopen(marker, "wb");
				if (f) {
					fwrite(version, 1, strlen(version), f);
					fclose(f);
					log_add("version marker written");
				} else {
					log_add("could not write the version marker");
				}
				printf("\n  Done.\n");
			} else if (!present) {
				fail("Could not find the app on the SD card:", path);
			}
		}
	}

	f = fopen(path, "rb");
	if (!f) fail("Could not find the app on the SD card:", path);
	fseek(f, 0, SEEK_END);
	size = ftell(f);
	fseek(f, 0, SEEK_SET);
	log_add("program: %ld bytes", size);
	if (size < (long)sizeof(dol_header) || size > 24 * 1024 * 1024) {
		fclose(f);
		fail("That file is not a usable program:", path);
	}

	/* Park the file at the top of MEM2, far from where programs load. */
	top = (u32)SYS_GetArena2Hi();
	base = (top - (u32)size) & ~31u;
	if (base < (u32)SYS_GetArena2Lo() + 0x10000) {
		fclose(f);
		fail("Not enough memory to load the app:", path);
	}
	if (fread((void *)base, 1, size, f) != (size_t)size) {
		fclose(f);
		fail("Could not read the whole app:", path);
	}
	fclose(f);

	if (!dol_validate((const dol_header *)base, (u32)size, base, base + (u32)size))
		fail("That file is not a valid Wii program:", path);

	/* Programs find their own folder from argv[0], exactly as under the Homebrew Channel. */
	dol_set_argv(path);
	log_add("argv: system=%p magic=%08x block=%08x", __system_argv, (unsigned)__system_argv->argvMagic, *(volatile u32 *)0x80001800);
	log_add("launching");
	log_close();

	fatUnmount("sd");
	/* Restart IOS so the program starts from a clean slate instead of inheriting our open handles. */
	IOS_ReloadIOS(IOS_GetVersion());

	dol_launch((const dol_header *)base);
	return 0; /* not reached */
}
