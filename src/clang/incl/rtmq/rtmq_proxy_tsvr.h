#if !defined(__RTMQ_PROXY_TSVR_H__)
#define __RTMQ_PROXY_TSVR_H__

#include "log.h"
#include "slab.h"
#include "list.h"
#include "avl_tree.h"
#include "rtmq_comm.h"
#include "thread_pool.h"

typedef struct _rtmq_proxy_sck_t rtmq_proxy_sck_t;
typedef int (*rtmq_proxy_socket_recv_cb_t)(void *ctx, void *obj, rtmq_proxy_sck_t *sck);
typedef int (*rtmq_proxy_socket_send_cb_t)(void *ctx, void *obj, rtmq_proxy_sck_t *sck);

/* 套接字信息 */
struct _rtmq_proxy_sck_t
{
    int fd;                             /* 套接字ID */
    time_t wrtm;                        /* 最近写入操作时间 */
    time_t rdtm;                        /* 最近读取操作时间 */

#define RTMQ_KPALIVE_STAT_UNKNOWN   (0) /* 未知状态 */
#define RTMQ_KPALIVE_STAT_SENT      (1) /* 已发送保活 */
#define RTMQ_KPALIVE_STAT_SUCC      (2) /* 保活成功 */
    int kpalive;                        /* 保活状态
                                            0: 未知状态
                                            1: 已发送保活
                                            2: 保活成功 */
    list_t *mesg_list;                  /* 发送链表 */

    rtmq_snap_t recv;                   /* 接收快照 */
    wiov_t send;                        /* 发送信息 */

    rtmq_proxy_socket_recv_cb_t recv_cb;/* 接收回调 */
    rtmq_proxy_socket_send_cb_t send_cb;/* 发送回调 */
};

#define rtmq_set_kpalive_stat(sck, _stat) (sck)->kpalive = (_stat)

/* SND线程上下文 */
typedef struct
{
    int id;                             /* 对象ID */
    void *ctx;                          /* 存储rtmq_proxy_t对象 */
    queue_t *sendq;                     /* 发送缓存 */
    log_cycle_t *log;                   /* 日志对象 */
    char ipaddr[IP_ADDR_MAX_LEN];       /* IP地址 */
    int port;                           /* 服务端端口 */

    int epid;                           /* Epoll描述符 */
    struct epoll_event *events;         /* Event最大数 */

    int fd[2];                          /* 通信FD */
    int cmd_fd;                         /* 命令通信FD */
    rtmq_proxy_sck_t sck;               /* 数据传输套接字 */
    rtmq_proxy_sck_t cmd_sck;           /* 命令通信套接字 */

    int max;                            /* 套接字最大值 */
    fd_set rset;                        /* 读集合 */
    fd_set wset;                        /* 写集合 */

    /* 统计信息 */
    uint64_t recv_total;                /* 获取的数据总条数 */
    uint64_t err_total;                 /* 错误的数据条数 */
    uint64_t drop_total;                /* 丢弃的数据条数 */
} rtmq_proxy_tsvr_t;

#endif /*__RTMQ_PROXY_TSVR_H__*/
