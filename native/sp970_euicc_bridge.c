/* SPDX-License-Identifier: AGPL-3.0-or-later
 * Pipe-only ISD-R transport for the stock ARM32 QMI ABI. The HTTP layer never
 * accepts arbitrary APDUs. Only ES10 STORE DATA and GET RESPONSE are accepted.
 * Keep stdout private: profile metadata is returned to the caller over a pipe.
 * Old libqmi's logical-channel response parser assumes channel TLV comes first;
 * this modem returns FCI and SW before it, so parse the raw TLVs by tag instead.
 */
#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
typedef int (*init_fn)(void*,void*);
typedef int (*release_fn)(int);
typedef int (*conn_fn)(const char*,int*);
typedef int (*client_fn)(const char*,void*,void*,int*);
typedef int (*client_release_fn)(int,int*);
typedef int (*sync_fn)(int,int,unsigned long,unsigned char*,int,unsigned char*,int*,int,int,int*);
static int flight(void) {
    FILE *f=popen("/system/bin/settings get global airplane_mode_on","r"); char s[16]={0};
    if(!f)return 0;
    int ok=fgets(s,sizeof(s),f)!=NULL; int rc=pclose(f); return ok && !rc && atoi(s)==1;
}
static int find(unsigned char *p,int size,unsigned tag,unsigned char **value) {
    for(int i=0;i+3<=size;) {
        unsigned t=p[i++],n=p[i]|((unsigned)p[i+1]<<8);i+=2;
        if(n>(unsigned)(size-i))return -1;
        if(t==tag){*value=p+i;return (int)n;} i+=(int)n;
    }return -1;
}
static int unhex(char *s,unsigned char *out,int cap) {
    size_t n=strcspn(s,"\r\n"); if(n%2||n>(size_t)cap*2)return -1;
    for(size_t i=0;i<n;i+=2){unsigned a=s[i],b=s[i+1];
        a=a>='0'&&a<='9'?a-'0':a>='a'&&a<='f'?a-'a'+10:a>='A'&&a<='F'?a-'A'+10:255;
        b=b>='0'&&b<='9'?b-'0':b>='a'&&b<='f'?b-'a'+10:b>='A'&&b<='F'?b-'A'+10:255;
        if(a>15||b>15)return -1;out[i/2]=(a<<4)|b;
    }return (int)n/2;
}
int main(void) {
    if(!flight()){puts("ERR flight_mode_required");return 2;}
    alarm(90);setvbuf(stdout,NULL,_IONBF,0);
    void *lib=dlopen("/system/vendor/lib/libqmi.so",RTLD_NOW);if(!lib)return 3;
    init_fn init=(init_fn)dlsym(lib,"qmi_init");release_fn release=(release_fn)dlsym(lib,"qmi_release");
    conn_fn connect=(conn_fn)dlsym(lib,"qmi_dev_connection_init");client_fn client_init=(client_fn)dlsym(lib,"qmi_uim_srvc_init_client");
    client_release_fn client_release=(client_release_fn)dlsym(lib,"qmi_uim_srvc_release_client");
    sync_fn sync=(sync_fn)dlsym(lib,"qmi_service_send_msg_sync");
    if(!init||!release||!connect||!client_init||!client_release||!sync)return 4;
    int handle=init(NULL,NULL),error=0,client=-1,result=1,length;unsigned char channel=0;
    unsigned char response[32768],request[1024],*v=NULL;
    if(handle<0||connect("rmnet0",&error)<0)goto done;
    client=client_init("rmnet0",NULL,NULL,&error);if(client<0)goto done;
    unsigned char open[]={1,1,0,1,0x10,17,0,16,0xa0,0,0,5,0x59,0x10,0x10,0xff,0xff,0xff,0xff,0x89,0,0,1,0};
    length=sizeof(response);
    int rc=sync(client,11,0x3f,open,sizeof(open),response,&length,sizeof(response),5,&error);
    if(rc||find(response,length,0x10,&v)!=1||v[0]<1||v[0]>19){puts("ERR isdr_unavailable");goto done;}
    channel=v[0];printf("READY %u\n",channel);
    char line[2048];unsigned char command[512];
    while(fgets(line,sizeof(line),stdin)) {
        if(!strncmp(line,"CLOSE",5)){result=0;break;}
        if(!flight()){puts("ERR flight_mode_required");break;}
        int n=unhex(line,command,sizeof(command));
        unsigned char cla=channel<4?(0x80|channel):(0xc0|(channel-4));
        if(n<4||n>261||command[0]!=cla||(command[1]!=0xe2&&command[1]!=0xc0)) {puts("ERR invalid_apdu");break;}
        unsigned size=(unsigned)n+2;
        unsigned char prefix[]={1,1,0,1,2,(unsigned char)size,(unsigned char)(size>>8),(unsigned char)n,(unsigned char)(n>>8)};
        memcpy(request,prefix,sizeof(prefix));memcpy(request+sizeof(prefix),command,n);
        int offset=(int)sizeof(prefix)+n;
        request[offset++]=0x10;request[offset++]=1;request[offset++]=0;request[offset++]=channel;
        length=sizeof(response);rc=sync(client,11,0x3b,request,offset,response,&length,sizeof(response),5,&error);
        int total=rc?-1:find(response,length,0x10,&v);
        if(total<4){puts("ERR apdu_failed");break;}
        unsigned bytes=v[0]|((unsigned)v[1]<<8);
        if(bytes<2||bytes>(unsigned)total-2){puts("ERR invalid_response");break;}
        printf("OK ");for(unsigned i=0;i<bytes;i++)printf("%02x",v[2+i]);puts("");
        memset(command,0,sizeof(command));memset(response,0,sizeof(response));
    }
done:
    if(channel){unsigned char close[]={1,1,0,1,0x11,1,0,channel};length=sizeof(response);(void)sync(client,11,0x3f,close,sizeof(close),response,&length,sizeof(response),5,&error);}
    if(client>=0)client_release(client,&error);
    if(handle>=0)release(handle);
    alarm(0);return result;
}
