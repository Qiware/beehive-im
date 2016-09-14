#if !defined(__ACC_RSVR_H__)
#define __ACC_RSVR_H__

#include "list.h"
#include "mesg.h"
#include "queue.h"
#include "access.h"
#include "rb_tree.h"

#define ACC_TMOUT_MSEC       (1000)  /* 超时(豪秒) */

#define ACC_EVENT_MAX_NUM    (8192)  /* 事件最大数 */
#define ACC_SCK_HASH_MOD     (7)     /* 套接字哈希长度 */

typedef struct
{
    int id;                         /* 对象ID */

    log_cycle_t *log;               /* 日志对象 */

    int epid;                       /* epoll描述符 */
    int fds;                        /* 处于激活状态的套接字数 */
    struct epoll_event *events;     /* Event最大数 */

    socket_t cmd_sck;               /* 命令套接字 */
    unsigned int conn_total;        /* 当前连接数 */

    time_t ctm;                     /* 当前时间 */
    time_t scan_tm;                 /* 前一次超时扫描的时间 */
    uint32_t recv_seq;              /* 业务接收序列号(此值将用于生成系统流水号) */
} acc_rsvr_t;

/* 套接字信息 */
typedef struct
{
    uint64_t sid;                   /* SCK序列号(主键) */
    int aid;                        /* 接收服务ID */
    bool is_cmd_sck;                /* 是否是命令套接字(false:否 true:是) */

    mesg_header_t *head;            /* 报头起始地址 */
    void *body;                     /* Body */
    list_t *send_list;              /* 发送链表 */
} acc_socket_extra_t;

void *acc_rsvr_routine(void *_ctx);

int acc_rsvr_init(acc_cntx_t *ctx, acc_rsvr_t *agent, int idx);
int acc_rsvr_destroy(acc_rsvr_t *agent);

#endif /*__ACC_RSVR_H__*/
