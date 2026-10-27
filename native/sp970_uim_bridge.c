/* SPDX-License-Identifier: AGPL-3.0-or-later
 * Stock-SP970-B UIM bridge. Protocol data on stdout is private:
 * consume it only through a pipe, never a terminal or diagnostic log.
 * identity -> OK IMSI ICCID MNC-length AID-hex
 * auth <- expected-IMSI RAND-hex AUTN-hex; -> OK USIM-response-hex
 * smsp -> OK record-hex (EF_SMSP record 1, read only)
 * Only UIM status, transparent/record reads, 3G AKA and cleanup are available.
 * No PIN, SIM/profile writes, power or radio commands are implemented.
 */
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef struct { uint16_t length; unsigned char *bytes; } bytes;
typedef struct { int type; bytes aid; } session;
typedef struct { uint16_t id; bytes path; } file_id;
typedef struct { session session; file_id file; uint16_t offset, length; } read_params;
typedef struct { session session; int context; bytes data; } auth_params;
typedef struct {
    int system_error, qmi_error, response_id;
    unsigned char sw1, sw2;
    bytes content;
} data_response;
typedef int (*init_fn)(void *, void *);
typedef int (*release_fn)(int);
typedef int (*connect_fn)(const char *, int *);
typedef int (*client_fn)(const char *, void *, void *, int *);
typedef int (*client_release_fn)(int, int *);
typedef int (*status_fn)(int, void *, void *, void *);
typedef int (*request_fn)(int, const void *, void *, void *, void *);

_Static_assert(sizeof(void *) == 4, "stock ABI is ARM32");
_Static_assert(sizeof(read_params) == 28, "unexpected read ABI");
_Static_assert(sizeof(auth_params) == 24, "unexpected authentication ABI");
_Static_assert(sizeof(data_response) == 24, "unexpected response ABI");

static void wipe(void *p, size_t n) {
    volatile unsigned char *v = p;
    while (n--) *v++ = 0;
}
static uint32_t u32(const unsigned char *p) {
    uint32_t v; memcpy(&v, p, 4); return v;
}
static int flight_mode(void) {
    FILE *f = popen("/system/bin/settings get global airplane_mode_on", "r");
    if (!f) return 0;
    char line[32] = {0};
    char *value = fgets(line, sizeof(line), f);
    int rc = pclose(f);
    return value && rc == 0 && atoi(line) == 1;
}
static int unhex(const char *s, unsigned char *out, size_t n) {
    if (strlen(s) != n * 2) return 0;
    for (size_t i = 0; i < n; ++i) {
        unsigned int a = (unsigned char)s[i*2], b = (unsigned char)s[i*2+1];
        a = a >= '0' && a <= '9' ? a-'0' : a >= 'a' && a <= 'f' ? a-'a'+10 : a >= 'A' && a <= 'F' ? a-'A'+10 : 255;
        b = b >= '0' && b <= '9' ? b-'0' : b >= 'a' && b <= 'f' ? b-'a'+10 : b >= 'A' && b <= 'F' ? b-'A'+10 : 255;
        if (a > 15 || b > 15) return 0;
        out[i] = (a << 4) | b;
    }
    return 1;
}
static void hex(const unsigned char *data, size_t n) {
    for (size_t i = 0; i < n; ++i) printf("%02x", data[i]);
}
static int read_file(int client, request_fn read_api, session sess, uint16_t id,
                     unsigned char *out, uint16_t length) {
    unsigned char path[] = {0x3f, 0x00, 0x7f, 0xff};
    read_params p = {0};
    p.session = sess;
    p.file.id = id;
    p.file.path = (bytes){id == 0x2fe2 ? 2 : 4, path};
    p.length = length;
    unsigned char *storage = calloc(1, 32768);
    if (!storage) return 0;
    data_response *r = (data_response *)storage;
    int rc = read_api(client, &p, NULL, NULL, r);
    int ok = rc == 0 && !r->system_error && !r->qmi_error &&
             r->sw1 == 0x90 && r->sw2 == 0 && r->content.bytes &&
             r->content.length == length;
    fprintf(stderr, "read file=%04x rc=%d qmi=%d sw=%02x%02x length=%u\n",
            id, rc, r->qmi_error, r->sw1, r->sw2, r->content.length);
    if (ok) memcpy(out, r->content.bytes, length);
    if (rc == 0 && r->content.bytes) {
        if (r->content.length <= 32768) wipe(r->content.bytes, r->content.length);
        free(r->content.bytes);
    }
    wipe(storage, 32768); free(storage);
    return ok;
}
static int decode_imsi(const unsigned char raw[9], char out[32]) {
    if (raw[0] < 2 || raw[0] > 8 || raw[1] >> 4 > 9) return 0;
    unsigned int n = 0;
    out[n++] = '0' + (raw[1] >> 4);
    for (unsigned int i = 2; i <= raw[0]; ++i) {
        if ((raw[i] & 15) <= 9) out[n++] = '0' + (raw[i] & 15);
        if ((raw[i] >> 4) <= 9) out[n++] = '0' + (raw[i] >> 4);
    }
    out[n] = 0;
    return n == 15 && strncmp(out, "23487", 5) == 0;
}

int main(int argc, char **argv) {
    int result = 1, lib_handle = -1, client = -1, error = 0;
    unsigned char status[32768] = {0}, raw_imsi[9] = {0}, raw_iccid[10] = {0}, ad[4] = {0};
    char imsi[32] = {0}, iccid[32] = {0}, input[192] = {0};
    release_fn release = NULL;
    client_release_fn release_client = NULL;
    alarm(25);
    if (argc != 2 || (strcmp(argv[1], "identity") && strcmp(argv[1], "auth") && strcmp(argv[1], "smsp"))) {
        fprintf(stderr, "Only identity, smsp and auth are supported\n"); return 2;
    }
    if (!flight_mode()) { fprintf(stderr, "flight-mode guard failed\n"); return 3; }
    void *lib = dlopen("/system/vendor/lib/libqmi.so", RTLD_NOW);
    if (!lib) { fprintf(stderr, "vendor QMI library unavailable\n"); return 4; }
    init_fn init = (init_fn)dlsym(lib, "qmi_init");
    release = (release_fn)dlsym(lib, "qmi_release");
    connect_fn connect_api = (connect_fn)dlsym(lib, "qmi_dev_connection_init");
    client_fn init_client = (client_fn)dlsym(lib, "qmi_uim_srvc_init_client");
    release_client = (client_release_fn)dlsym(lib, "qmi_uim_srvc_release_client");
    status_fn get_status = (status_fn)dlsym(lib, "qmi_uim_get_card_status");
    request_fn read_api = (request_fn)dlsym(lib, "qmi_uim_read_transparent");
    request_fn auth_api = (request_fn)dlsym(lib, "qmi_uim_authenticate");
    if (!init || !release || !connect_api || !init_client || !release_client || !get_status || !read_api || !auth_api) goto done;
    lib_handle = init(NULL, NULL);
    if (lib_handle < 0 || connect_api("rmnet0", &error) < 0) goto done;
    client = init_client("rmnet0", NULL, NULL, &error);
    if (client < 0 || get_status(client, NULL, NULL, status) < 0 || u32(status) || u32(status+4)) goto done;
    if (status[20] < 1 || u32(status+24) != 1 || !status[40] ||
        u32(status+44) != 2 || u32(status+48) != 7 || !status[62] || status[62] > 32) goto done;
    session app = {4, {status[62], status+63}};
    if (!read_file(client, read_api, app, 0x6f07, raw_imsi, 9) || !decode_imsi(raw_imsi, imsi)) goto done;
    if (!strcmp(argv[1], "identity")) {
        session card = {6, {0, NULL}};
        if (!read_file(client, read_api, app, 0x6fad, ad, 4) ||
            !read_file(client, read_api, card, 0x2fe2, raw_iccid, 10)) goto done;
        unsigned int n = 0;
        for (unsigned int i = 0; i < 10; ++i) {
            if ((raw_iccid[i]&15) <= 9) iccid[n++] = '0'+(raw_iccid[i]&15);
            if ((raw_iccid[i]>>4) <= 9) iccid[n++] = '0'+(raw_iccid[i]>>4);
        }
        unsigned int mnc = ad[3] & 15;
        if (n < 18 || (mnc != 2 && mnc != 3)) goto done;
        printf("OK %s %s %u ", imsi, iccid, mnc);
        hex(app.aid.bytes, app.aid.length); putchar('\n');
        result = 0;
    } else if (!strcmp(argv[1], "smsp")) {
        request_fn record_api = (request_fn)dlsym(lib, "qmi_uim_read_record");
        unsigned char path[] = {0x3f, 0x00, 0x7f, 0xff};
        /* Same vendor ABI as transparent reads; offset is the record number. */
        read_params p = {app, {0x6f42, {4, path}}, 1, 0};
        unsigned char *storage = calloc(1, 32768);
        if (!storage || !record_api) { free(storage); goto done; }
        data_response *r = (data_response *)storage;
        int rc = record_api(client, &p, NULL, NULL, r);
        fprintf(stderr, "read EF_SMSP record=1 rc=%d qmi=%d sw=%02x%02x length=%u\n",
                rc, r->qmi_error, r->sw1, r->sw2, r->content.length);
        if (rc == 0 && !r->system_error && !r->qmi_error && r->sw1 == 0x90 && r->sw2 == 0 &&
            r->content.bytes && r->content.length >= 28 && r->content.length <= 255) {
            printf("OK "); hex(r->content.bytes, r->content.length); putchar('\n'); result = 0;
        }
        if (rc == 0 && r->content.bytes) {
            if (r->content.length <= 32768) wipe(r->content.bytes, r->content.length);
            free(r->content.bytes);
        }
        wipe(storage, 32768); free(storage);
    } else {
        char expected[32] = {0}, rand_hex[34] = {0}, autn_hex[34] = {0}, extra[2] = {0};
        unsigned char data[34] = {0}; data[0] = data[17] = 16;
        if (!fgets(input, sizeof(input), stdin) ||
            sscanf(input, "%31s %33s %33s %1s", expected, rand_hex, autn_hex, extra) != 3 ||
            strcmp(expected, imsi) || !unhex(rand_hex, data+1, 16) || !unhex(autn_hex, data+18, 16)) {
            fprintf(stderr, "authentication input or SIM-identity guard failed\n"); goto done;
        }
        auth_params p = {app, 3, {34, data}};
        unsigned char *storage = calloc(1, 32768);
        if (!storage) goto done;
        data_response *r = (data_response *)storage;
        int rc = auth_api(client, &p, NULL, NULL, r);
        fprintf(stderr, "auth rc=%d qmi=%d sw=%02x%02x length=%u\n",
                rc, r->qmi_error, r->sw1, r->sw2, r->content.length);
        if (rc == 0 && !r->system_error && !r->qmi_error && r->sw1 == 0x90 && r->sw2 == 0 &&
            r->content.bytes && r->content.length >= 2 && r->content.length <= 128) {
            printf("OK "); hex(r->content.bytes, r->content.length); putchar('\n'); result = 0;
        }
        if (rc == 0 && r->content.bytes) {
            if (r->content.length <= 32768) wipe(r->content.bytes, r->content.length);
            free(r->content.bytes);
        }
        wipe(storage, 32768); free(storage);
        wipe(data, sizeof(data)); wipe(expected, sizeof(expected));
        wipe(rand_hex, sizeof(rand_hex)); wipe(autn_hex, sizeof(autn_hex));
    }
done:
    if (result) puts("ERR uim-operation-failed");
    fflush(stdout);
    if (client >= 0 && release_client) release_client(client, &error);
    if (lib_handle >= 0 && release) release(lib_handle);
    wipe(imsi, sizeof(imsi)); wipe(iccid, sizeof(iccid)); wipe(input, sizeof(input));
    wipe(raw_imsi, sizeof(raw_imsi)); wipe(raw_iccid, sizeof(raw_iccid));
    alarm(0);
    return result;
}
