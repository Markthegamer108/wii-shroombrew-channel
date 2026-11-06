/*
 * Reading the app out of the channel's own content and unpacking it.
 * Original code for Shroombrew; the ZIP layout parsed here is the public format
 * (PKWARE APPNOTE), inflated with zlib.
 *
 * The package is read once, front to back, through a 64 KB window: each entry is
 * found by its local header and inflated straight onto the SD card. Nothing is
 * staged in memory, so the size of an app is limited by the card, not by RAM.
 */
#include <gccore.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <zlib.h>

#include "payload.h"

#define MAX_ONE_FILE 0xF0000000u /* stays under the 4 GB FAT32 file limit */
#define MAX_TOTAL 0xF0000000ull
#define MAX_NAME 190
#define PROGRESS_STEP (256u * 1024)

enum { E_BADZIP = -1, E_UNSAFE = -2, E_UNSUPPORTED = -3, E_INFLATE = -4, E_WRITE = -5, E_TOOBIG = -6, E_EMPTY = -7, E_READ = -8 };

const char *payload_error_text(int c) {
	switch (c) {
	case E_BADZIP: return "The packaged app is damaged.";
	case E_UNSAFE: return "The package contains an unsafe file name.";
	case E_UNSUPPORTED: return "The package uses an unsupported ZIP feature.";
	case E_INFLATE: return "Could not decompress a file.";
	case E_WRITE: return "Could not write to the SD card (is it full or locked?).";
	case E_TOOBIG: return "The package is too large.";
	case E_EMPTY: return "The package has no files for the SD card.";
	case E_READ: return "Could not read the app from the channel.";
	}
	return "Unknown error.";
}

static u16 rd16(const u8 *p) { return (u16)(p[0] | (p[1] << 8)); }
static u32 rd32(const u8 *p) { return (u32)p[0] | ((u32)p[1] << 8) | ((u32)p[2] << 16) | ((u32)p[3] << 24); }

/* ---- the package as a forward-only byte stream ---- */

static s32 es_fd = -1;
static FILE *src_file = NULL;
static u32 src_total, src_fetched, consumed;
static u8 rbuf[0x10000] ATTRIBUTE_ALIGN(32);
static u32 rlen, roff;
static u8 outbuf[32768];

static int fill(void) {
	u32 want = src_total - src_fetched, ask;
	s32 r;
	if (!want) return 0;
	if (want > sizeof(rbuf)) want = sizeof(rbuf);
	if (src_file) {
		r = (s32)fread(rbuf, 1, want, src_file);
	} else {
		ask = (want + 31) & ~31u; /* ES wants 32-byte multiples */
		r = ES_ReadContent(es_fd, rbuf, ask);
	}
	if (r <= 0) return 0;
	rlen = (u32)r > want ? want : (u32)r;
	roff = 0;
	src_fetched += rlen;
	return 1;
}

/* Copies n bytes to dst, or just drops them when dst is NULL. */
static int take(u8 *dst, u32 n) {
	while (n) {
		u32 c;
		if (roff == rlen && !fill()) return 0;
		c = rlen - roff;
		if (c > n) c = n;
		if (dst) {
			memcpy(dst, rbuf + roff, c);
			dst += c;
		}
		roff += c;
		consumed += c;
		n -= c;
	}
	return 1;
}

static void reset_stream(u32 total) {
	src_total = total;
	src_fetched = consumed = rlen = roff = 0;
}

u32 payload_open(void) {
	u32 size;
	payload_close();
	es_fd = ES_OpenContent(PAYLOAD_CONTENT_INDEX);
	if (es_fd < 0) {
		es_fd = -1;
		return 0;
	}
	size = (u32)ES_SeekContent(es_fd, 0, 2 /* SEEK_END */);
	if ((s32)size <= 0 || ES_SeekContent(es_fd, 0, 0) < 0) {
		payload_close();
		return 0;
	}
	reset_stream(size);
	return size;
}

/* Development and repair path: take the package from the SD card instead of the
 * channel's own content. Used by tools-src/forwarder/test and handy when a
 * channel was built without one. */
u32 payload_open_file(const char *path) {
	long size;
	payload_close();
	src_file = fopen(path, "rb");
	if (!src_file) return 0;
	fseek(src_file, 0, SEEK_END);
	size = ftell(src_file);
	fseek(src_file, 0, SEEK_SET);
	if (size <= 0) {
		payload_close();
		return 0;
	}
	reset_stream((u32)size);
	return (u32)size;
}

void payload_close(void) {
	if (es_fd >= 0) ES_CloseContent(es_fd);
	es_fd = -1;
	if (src_file) fclose(src_file);
	src_file = NULL;
}

/* ---- unpacking ---- */

/* Only plain relative names under apps/ are allowed. */
static int safe_name(const char *n, u32 len) {
	u32 i;
	if (len == 0 || len > 180) return 0;
	if (n[0] == '/') return 0;
	for (i = 0; i < len; i++) {
		unsigned char c = (unsigned char)n[i];
		if (c < 0x20 || c == 0x7f || c == '\\' || c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|') return 0;
		if (c == '.' && (i == 0 || n[i - 1] == '/') && (i + 1 == len || n[i + 1] == '/' || (n[i + 1] == '.' && (i + 2 == len || n[i + 2] == '/')))) return 0; /* "." and ".." parts */
	}
	return len >= 5 && strncmp(n, "apps/", 5) == 0;
}

static void make_dirs(char *path) { /* path = "sd:/a/b/c" ; creates every level */
	char *p;
	for (p = path + 4; *p; p++) {
		if (*p == '/') {
			*p = 0;
			mkdir(path, 0777);
			*p = '/';
		}
	}
}

/* Stored entry: the bytes go straight from the window to the file. */
static int copy_stored(FILE *f, u32 n) {
	while (n) {
		u32 c;
		if (roff == rlen && !fill()) return E_READ;
		c = rlen - roff;
		if (c > n) c = n;
		if (fwrite(rbuf + roff, 1, c, f) != c) return E_WRITE;
		roff += c;
		consumed += c;
		n -= c;
	}
	return 0;
}

/* Deflated entry: feeds zlib from the window and writes what comes out. */
static int inflate_entry(FILE *f, u32 comp, u32 uncomp, void (*progress)(u32, u32), u32 *lastReport) {
	z_stream zs;
	u32 left = comp, produced = 0;
	int st, rc = 0;
	memset(&zs, 0, sizeof(zs));
	if (inflateInit2(&zs, -MAX_WBITS) != Z_OK) return E_INFLATE;
	do {
		u32 n;
		if (zs.avail_in == 0 && left) {
			u32 c;
			if (roff == rlen && !fill()) { rc = E_READ; break; }
			c = rlen - roff;
			if (c > left) c = left;
			zs.next_in = rbuf + roff;
			zs.avail_in = c;
			roff += c;
			consumed += c;
			left -= c;
		}
		zs.next_out = outbuf;
		zs.avail_out = sizeof(outbuf);
		st = inflate(&zs, Z_NO_FLUSH);
		if (st != Z_OK && st != Z_STREAM_END) { rc = E_INFLATE; break; }
		n = sizeof(outbuf) - zs.avail_out;
		produced += n;
		if (produced > uncomp) { rc = E_INFLATE; break; }
		if (n && fwrite(outbuf, 1, n, f) != n) { rc = E_WRITE; break; }
		if (progress && consumed - *lastReport >= PROGRESS_STEP) {
			*lastReport = consumed;
			progress(consumed, src_total);
		}
	} while (st != Z_STREAM_END);
	inflateEnd(&zs);
	if (rc) return rc;
	if (produced != uncomp) return E_INFLATE;
	if (left && !take(NULL, left)) return E_READ; /* trailing bytes of this entry */
	return 0;
}

int payload_extract(void (*progress)(u32, u32)) {
	u32 files = 0, lastReport = 0;
	u64 total = 0;
	for (;;) {
		u8 h[30];
		char name[MAX_NAME + 8], path[4 + MAX_NAME + 8];
		u16 flags, method, nl, el;
		u32 comp, uncomp, sig;
		FILE *f;
		int rc;

		if (!take(h, 4)) return E_BADZIP;
		sig = rd32(h);
		if (sig == 0x02014b50 || sig == 0x06054b50 || sig == 0x06064b50) break; /* central directory: all entries seen */
		if (sig != 0x04034b50) return E_BADZIP;
		if (!take(h + 4, 26)) return E_BADZIP;
		flags = rd16(h + 6);
		method = rd16(h + 8);
		comp = rd32(h + 18);
		uncomp = rd32(h + 22);
		nl = rd16(h + 26);
		el = rd16(h + 28);
		if (flags & 1) return E_UNSUPPORTED; /* encrypted */
		if (flags & 8) return E_UNSUPPORTED; /* sizes follow the data, so the entry cannot be streamed */
		if (comp == 0xffffffffu || uncomp == 0xffffffffu) return E_UNSUPPORTED; /* zip64 */
		if (nl == 0 || nl > MAX_NAME) return E_UNSAFE;
		if (!take((u8 *)name, nl)) return E_BADZIP;
		name[nl] = 0;
		if (el && !take(NULL, el)) return E_BADZIP;

		if (!safe_name(name, nl)) {
			/* entries outside apps/ (such as a top-level README) are ignored, but unsafe ones are fatal */
			u32 k;
			int dotdot = 0;
			for (k = 0; k + 1 < nl; k++) if (name[k] == '.' && name[k + 1] == '.') dotdot = 1;
			if (dotdot || name[0] == '/' || memchr(name, '\\', nl) || memchr(name, ':', nl)) return E_UNSAFE;
			if (!take(NULL, comp)) return E_BADZIP;
			continue;
		}

		memcpy(path, "sd:/", 4);
		memcpy(path + 4, name, nl + 1);
		if (name[nl - 1] == '/') {
			make_dirs(path);
			mkdir(path, 0777);
			if (!take(NULL, comp)) return E_BADZIP;
			continue;
		}

		if (method != 0 && method != 8) return E_UNSUPPORTED;
		if (uncomp > MAX_ONE_FILE || comp > MAX_ONE_FILE) return E_TOOBIG;
		total += uncomp;
		if (total > MAX_TOTAL) return E_TOOBIG;
		if (method == 0 && uncomp != comp) return E_BADZIP;

		make_dirs(path);
		f = fopen(path, "wb");
		if (!f) return E_WRITE;
		rc = method == 0 ? copy_stored(f, comp) : inflate_entry(f, comp, uncomp, progress, &lastReport);
		if (fclose(f) != 0 && !rc) rc = E_WRITE;
		if (rc) return rc;
		files++;
		if (progress) {
			lastReport = consumed;
			progress(consumed, src_total);
		}
	}
	if (!files) return E_EMPTY;
	return (int)files;
}
