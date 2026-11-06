/* Test program for the forwarder: proves it was started, with the right argv,
 * a clean bss and working SD access, by writing sd:/forwarder-test.txt. */
#include <gccore.h>
#include <fat.h>
#include <stdio.h>
#include <string.h>

static int bssProbe[4096];            /* must start zeroed (the stub clears bss) */
static int dataProbe[4] = {1, 2, 3, 4}; /* initialised data must have been copied */

int main(int argc, char **argv) {
	int bssOK = 1, dataOK = (dataProbe[0] == 1 && dataProbe[3] == 4), i;
	for (i = 0; i < 4096; i++) if (bssProbe[i]) bssOK = 0;

	int fat = fatInitDefault();
	if (fat) {
		FILE *f = fopen("sd:/forwarder-test.txt", "wb");
		if (f) {
			fprintf(f, "started=yes\nargc=%d\nargv0=%s\nbss=%s\ndata=%s\n", argc, argc > 0 && argv ? argv[0] : "(none)", bssOK ? "ok" : "BAD", dataOK ? "ok" : "BAD");
			fclose(f);
		}
	}
	SYS_ResetSystem(SYS_POWEROFF, 0, 0);
	return 0;
}
