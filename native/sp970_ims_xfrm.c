/* SPDX-License-Identifier: AGPL-3.0-or-later
 * Small XFRM Netlink adapter for KitKat's ip without batch support.
 * Input, including integrity keys, is stdin only. No flush/update operation.
 */
#include <arpa/inet.h>
#include <errno.h>
#include <linux/netlink.h>
#include <linux/xfrm.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <unistd.h>

static void wipe(void *p, size_t n) {
    volatile unsigned char *v = p;
    while (n--) *v++ = 0;
}
static int address(char *text, xfrm_address_t *out, unsigned short *family) {
    char *slash = strchr(text, '/');
    if (slash) {
        if (strcmp(slash, "/32") && strcmp(slash, "/128")) return 0;
        *slash = 0;
    }
    *family = strchr(text, ':') ? AF_INET6 : AF_INET;
    if (slash && atoi(slash+1) != (*family == AF_INET6 ? 128 : 32)) return 0;
    return inet_pton(*family, text, out) == 1;
}
static void lifetime(struct xfrm_lifetime_cfg *l) {
    l->soft_byte_limit = l->hard_byte_limit = XFRM_INF;
    l->soft_packet_limit = l->hard_packet_limit = XFRM_INF;
    l->hard_add_expires_seconds = 3600;
}
static int attr(struct nlmsghdr *h, size_t capacity, unsigned short type, const void *data, size_t n) {
    size_t pos = NLMSG_ALIGN(h->nlmsg_len), size = NLA_HDRLEN + n;
    if (pos + NLA_ALIGN(size) > capacity) return 0;
    struct nlattr *a = (struct nlattr *)((unsigned char *)h + pos);
    a->nla_type = type; a->nla_len = size;
    memcpy((unsigned char *)a + NLA_HDRLEN, data, n);
    h->nlmsg_len = pos + NLA_ALIGN(size);
    return 1;
}
static int request(struct nlmsghdr *h) {
    int fd = socket(AF_NETLINK, SOCK_RAW, NETLINK_XFRM);
    if (fd < 0) return errno;
    struct sockaddr_nl local = { .nl_family = AF_NETLINK }, kernel = { .nl_family = AF_NETLINK };
    struct timeval timeout = {3, 0};
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout));
    if (bind(fd, (struct sockaddr *)&local, sizeof(local)) < 0) { int e=errno; close(fd); return e; }
    h->nlmsg_seq = 1;
    h->nlmsg_flags = NLM_F_REQUEST | NLM_F_ACK;
    if (h->nlmsg_type == XFRM_MSG_NEWSA || h->nlmsg_type == XFRM_MSG_NEWPOLICY)
        h->nlmsg_flags |= NLM_F_CREATE | NLM_F_EXCL;
    if (sendto(fd, h, h->nlmsg_len, 0, (struct sockaddr *)&kernel, sizeof(kernel)) < 0) { int e=errno; close(fd); return e; }
    unsigned char response[8192];
    int n = recv(fd, response, sizeof(response), 0), result = EPROTO;
    if (n < 0) result = errno;
    else {
        unsigned int remaining = (unsigned int)n;
        for (struct nlmsghdr *r=(struct nlmsghdr *)response; NLMSG_OK(r,remaining); r=NLMSG_NEXT(r,remaining)) {
            if (r->nlmsg_seq == 1 && r->nlmsg_type == NLMSG_ERROR && r->nlmsg_len >= NLMSG_LENGTH(sizeof(struct nlmsgerr))) {
                result = -((struct nlmsgerr *)NLMSG_DATA(r))->error;
                break;
            }
        }
    }
    wipe(response, sizeof(response)); close(fd); return result;
}
int main(int argc, char **argv) {
    alarm(6);
    if (argc != 2) return 2;
    int state = !strcmp(argv[1], "state-add") || !strcmp(argv[1], "state-delete");
    int policy = !strcmp(argv[1], "policy-add") || !strcmp(argv[1], "policy-delete");
    int add = !strcmp(argv[1], "state-add") || !strcmp(argv[1], "policy-add");
    if (!state && !policy) return 2;
    char input[512] = {0}, src[80]={0}, dst[80]={0}, dir[8]={0}, key[40]={0}, extra[2]={0};
    unsigned int spi=0, reqid=0, sport=0, dport=0;
    if (!fgets(input, sizeof(input), stdin)) return 2;
    int fields = state ? sscanf(input, "%79s %79s %u %u %39s %1s", src,dst,&spi,&reqid,key,extra) :
        sscanf(input, "%7s %79s %79s %u %u %u %u %1s", dir,src,dst,&sport,&dport,&spi,&reqid,extra);
    if ((state && fields != (add ? 5 : 4)) || (policy && fields != 7)) return 2;
    xfrm_address_t source={0}, destination={0}; unsigned short family=0, other=0;
    if (!address(src,&source,&family) || !address(dst,&destination,&other) || family!=other) return 2;
    if ((add && spi < 256) || (add && reqid < 1) ||
        (policy && (sport<1 || sport>65535 || dport<1 || dport>65535 || (strcmp(dir,"in") && strcmp(dir,"out"))))) return 2;
    unsigned char message[4096] = {0}; struct nlmsghdr *h=(struct nlmsghdr *)message;
    if (state && add) {
        struct xfrm_usersa_info *s=NLMSG_DATA(h);
        h->nlmsg_type=XFRM_MSG_NEWSA; h->nlmsg_len=NLMSG_LENGTH(sizeof(*s));
        s->id.daddr=destination; s->id.spi=htonl(spi); s->id.proto=IPPROTO_ESP;
        s->saddr=source; s->family=family; s->sel.family=family;
        s->reqid=reqid; s->mode=XFRM_MODE_TRANSPORT; s->replay_window=32; lifetime(&s->lft);
        unsigned char auth_data[sizeof(struct xfrm_algo_auth)+16] = {0};
        struct xfrm_algo_auth *auth=(struct xfrm_algo_auth *)auth_data;
        strcpy(auth->alg_name,"hmac(sha1)"); auth->alg_key_len=128; auth->alg_trunc_len=96;
        if (strlen(key)!=32) return 2;
        for (int i=0;i<16;i++) {
            unsigned int v; char hex[3]={key[i*2],key[i*2+1],0};
            if (sscanf(hex,"%2x",&v)!=1) return 2;
            auth->alg_key[i]=v;
        }
        struct xfrm_algo enc={0}; strcpy(enc.alg_name,"ecb(cipher_null)");
        if (!attr(h,sizeof(message),XFRMA_ALG_AUTH_TRUNC,auth_data,sizeof(auth_data)) ||
            !attr(h,sizeof(message),XFRMA_ALG_CRYPT,&enc,sizeof(enc))) return 2;
        wipe(auth_data,sizeof(auth_data));
    } else if (state) {
        struct xfrm_usersa_id *s=NLMSG_DATA(h);
        h->nlmsg_type=XFRM_MSG_DELSA; h->nlmsg_len=NLMSG_LENGTH(sizeof(*s));
        s->daddr=destination; s->spi=htonl(spi); s->proto=IPPROTO_ESP; s->family=family;
        if (!attr(h,sizeof(message),XFRMA_SRCADDR,&source,sizeof(source))) return 2;
    } else {
        struct xfrm_selector sel={0}; sel.saddr=source; sel.daddr=destination; sel.family=family;
        sel.prefixlen_s=sel.prefixlen_d=family==AF_INET6 ? 128 : 32;
        sel.proto=IPPROTO_TCP; sel.sport=htons(sport); sel.dport=htons(dport); sel.sport_mask=sel.dport_mask=0xffff;
        unsigned char direction=!strcmp(dir,"in") ? XFRM_POLICY_IN : XFRM_POLICY_OUT;
        if (add) {
            struct xfrm_userpolicy_info *p=NLMSG_DATA(h);
            h->nlmsg_type=XFRM_MSG_NEWPOLICY; h->nlmsg_len=NLMSG_LENGTH(sizeof(*p));
            p->sel=sel; p->dir=direction; p->priority=20970; p->action=XFRM_POLICY_ALLOW; p->share=XFRM_SHARE_ANY; lifetime(&p->lft);
            struct xfrm_user_tmpl t={0}; t.id.daddr=destination; t.id.spi=htonl(spi); t.id.proto=IPPROTO_ESP;
            t.saddr=source; t.family=family; t.reqid=reqid; t.mode=XFRM_MODE_TRANSPORT;
            t.aalgos=t.ealgos=t.calgos=0xffffffff;
            if (!attr(h,sizeof(message),XFRMA_TMPL,&t,sizeof(t))) return 2;
        } else {
            struct xfrm_userpolicy_id *p=NLMSG_DATA(h);
            h->nlmsg_type=XFRM_MSG_DELPOLICY; h->nlmsg_len=NLMSG_LENGTH(sizeof(*p)); p->sel=sel; p->dir=direction;
        }
    }
    int error=request(h);
    wipe(message,sizeof(message)); wipe(input,sizeof(input)); wipe(key,sizeof(key));
    if (error) { fprintf(stderr,"XFRM %s: %s\n",argv[1],strerror(error)); return 1; }
    puts("OK"); return 0;
}
