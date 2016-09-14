#include "sck.h"
#include "comm.h"
#include "list.h"
#include "mesg.h"
#include "redo.h"
#include "utils.h"
#include "access.h"
#include "mem_ref.h"
#include "command.h"
#include "xml_tree.h"
#include "hash_alg.h"
#include "acc_rsvr.h"
#include "thread_pool.h"

#define AGT_RSVR_DIST_POP_NUM   (1024)

static acc_rsvr_t *acc_rsvr_self(acc_cntx_t *ctx);
static int acc_rsvr_add_conn(acc_cntx_t *ctx, acc_rsvr_t *rsvr);
static int acc_rsvr_del_conn(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck);

static int acc_recv_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck);
static int acc_send_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck);

static int acc_rsvr_dist_send_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr);
static socket_t *acc_push_into_send_list(acc_cntx_t *ctx, acc_rsvr_t *rsvr, uint64_t sid, void *addr);

static int acc_rsvr_event_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr);
static int acc_rsvr_timeout_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr);

static int acc_rsvr_connection_cmp(const int *sid, const socket_t *sck);

/******************************************************************************
 **函数名称: acc_rsvr_routine
 **功    能: 运行接收线程
 **输入参数:
 **     _ctx: 全局对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.18 #
 ******************************************************************************/
void *acc_rsvr_routine(void *_ctx)
{
    acc_rsvr_t *rsvr;
    acc_cntx_t *ctx = (acc_cntx_t *)_ctx;

    nice(-20);

    /* > 获取代理对象 */
    rsvr = acc_rsvr_self(ctx);
    if (NULL == rsvr) {
        log_error(rsvr->log, "Get agent failed!");
        pthread_exit((void *)-1);
        return (void *)-1;
    }

    while (1) {
        /* > 等待事件通知 */
        rsvr->fds = epoll_wait(rsvr->epid, rsvr->events,
                ACC_EVENT_MAX_NUM, ACC_TMOUT_MSEC);
        if (rsvr->fds < 0) {
            if (EINTR == errno) {
                continue;
            }

            /* 异常情况 */
            log_error(rsvr->log, "errmsg:[%d] %s!", errno, strerror(errno));
            abort();
            return (void *)-1;
        }
        else if (0 == rsvr->fds) {
            rsvr->ctm = time(NULL);
            if (rsvr->ctm - rsvr->scan_tm > ACC_TMOUT_SCAN_SEC) {
                rsvr->scan_tm = rsvr->ctm;
                acc_rsvr_timeout_hdl(ctx, rsvr);
            }
            continue;
        }

        /* > 进行事件处理 */
        acc_rsvr_event_hdl(ctx, rsvr);
    }

    return NULL;
}

/* 命令接收处理 */
static int agt_rsvr_recv_cmd_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    cmd_data_t cmd;

    while (1) {
        /* > 接收命令 */
        if (unix_udp_recv(sck->fd, &cmd, sizeof(cmd)) < 0) {
            return ACC_SCK_AGAIN;
        }

        /* > 处理命令 */
        switch (cmd.type) {
            case CMD_ADD_SCK:
                if (acc_rsvr_add_conn(ctx, rsvr)) {
                    log_error(rsvr->log, "Add connection failed！");
                }
                break;
            case CMD_DIST_DATA:
                if (acc_rsvr_dist_send_data(ctx, rsvr)) {
                    log_error(rsvr->log, "Disturibute data failed！");
                }
                break;
            default:
                log_error(rsvr->log, "Unknown command type [%d]！", cmd.type);
                break;
        }
    }
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_init
 **功    能: 初始化Agent线程
 **输入参数:
 **     ctx: 全局信息
 **     rsvr: 接收对象
 **     idx: 线程索引
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.28 #
 ******************************************************************************/
int acc_rsvr_init(acc_cntx_t *ctx, acc_rsvr_t *rsvr, int idx)
{
    struct epoll_event ev;
    char path[FILE_NAME_MAX_LEN];
    acc_conf_t *conf = ctx->conf;
    socket_t *cmd_sck = &rsvr->cmd_sck;

    rsvr->id = idx;
    rsvr->log = ctx->log;
    rsvr->recv_seq = 0;

    do {
        /* > 创建epoll对象 */
        rsvr->epid = epoll_create(ACC_EVENT_MAX_NUM);
        if (rsvr->epid < 0) {
            log_error(rsvr->log, "errmsg:[%d] %s!", errno, strerror(errno));
            break;
        }

        rsvr->events = calloc(1, ACC_EVENT_MAX_NUM * sizeof(struct epoll_event));
        if (NULL == rsvr->events) {
            log_error(rsvr->log, "errmsg:[%d] %s!", errno, strerror(errno));
            break;
        }

        /* > 创建附加信息 */
        cmd_sck->extra = calloc(1, sizeof(acc_socket_extra_t));
        if (NULL == cmd_sck->extra) {
            log_error(rsvr->log, "Alloc from slab failed!");
            break;
        }

        /* > 创建命令套接字 */
        acc_rsvr_cmd_usck_path(conf, rsvr->id, path, sizeof(path));

        cmd_sck->fd = unix_udp_creat(path);
        if (cmd_sck->fd < 0) {
            log_error(rsvr->log, "errmsg:[%d] %s!", errno, strerror(errno));
            break;
        }

        ftime(&cmd_sck->crtm);
        cmd_sck->wrtm = cmd_sck->rdtm = cmd_sck->crtm.time;
        cmd_sck->recv_cb = (socket_recv_cb_t)agt_rsvr_recv_cmd_hdl;
        cmd_sck->send_cb = NULL;

        /* > 加入事件侦听 */
        memset(&ev, 0, sizeof(ev));

        ev.data.ptr = cmd_sck;
        ev.events = EPOLLIN | EPOLLET; /* 边缘触发 */

        epoll_ctl(rsvr->epid, EPOLL_CTL_ADD, cmd_sck->fd, &ev);

        return ACC_OK;
    } while(0);

    acc_rsvr_destroy(rsvr);
    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_rsvr_sck_dealloc
 **功    能: 释放SCK对象的空间
 **输入参数:
 **     pool: 内存池
 **     sck: 需要被释放的套接字对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 依次释放所有内存空间
 **注意事项: 
 **作    者: # Qifeng.zou # 2015.07.22 21:39:05 #
 ******************************************************************************/
static int acc_rsvr_sck_dealloc(void *pool, socket_t *sck)
{
    acc_socket_extra_t *extra = sck->extra;

    FREE(sck);
    list_destroy(extra->send_list, (mem_dealloc_cb_t)mem_dealloc, NULL);
    FREE(extra);

    return 0;
}

/******************************************************************************
 **函数名称: acc_rsvr_destroy
 **功    能: 销毁Agent线程
 **输入参数:
 **     rsvr: 接收对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 依次释放所有内存空间
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.18 #
 ******************************************************************************/
int acc_rsvr_destroy(acc_rsvr_t *rsvr)
{
    FREE(rsvr->events);
    CLOSE(rsvr->epid);
    CLOSE(rsvr->cmd_sck.fd);
    FREE(rsvr->cmd_sck.extra);
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_self
 **功    能: 获取代理对象
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 代理对象
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.26 #
 ******************************************************************************/
static acc_rsvr_t *acc_rsvr_self(acc_cntx_t *ctx)
{
    int id;
    acc_rsvr_t *rsvr;

    id = thread_pool_get_tidx(ctx->agents);
    if (id < 0) {
        return NULL;
    }

    rsvr = thread_pool_get_args(ctx->agents);

    return rsvr + id;
}

/******************************************************************************
 **函数名称: acc_rsvr_event_hdl
 **功    能: 事件通知处理
 **输入参数: 
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **输出参数: NONE
 **返    回: 代理对象
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.28 #
 ******************************************************************************/
static int acc_rsvr_event_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr)
{
    int idx, ret;
    socket_t *sck;

    rsvr->ctm = time(NULL);

    /* 1. 依次遍历套接字, 判断是否可读可写 */
    for (idx=0; idx<rsvr->fds; ++idx) {
        sck = (socket_t *)rsvr->events[idx].data.ptr;

        /* 1.1 判断是否可读 */
        if (rsvr->events[idx].events & EPOLLIN) {
            /* 接收网络数据 */
            ret = sck->recv_cb(ctx, rsvr, sck);
            if (ACC_SCK_AGAIN != ret) {
                log_info(rsvr->log, "Delete connection! fd:%d", sck->fd);
                acc_rsvr_del_conn(ctx, rsvr, sck);
                continue; /* 异常-关闭SCK: 不必判断是否可写 */
            }
        }

        /* 1.2 判断是否可写 */
        if (rsvr->events[idx].events & EPOLLOUT) {
            /* 发送网络数据 */
            ret = sck->send_cb(ctx, rsvr, sck);
            if (ACC_ERR == ret) {
                log_info(rsvr->log, "Delete connection! fd:%d", sck->fd);
                acc_rsvr_del_conn(ctx, rsvr, sck);
                continue; /* 异常: 套接字已关闭 */
            }
        }
    }

    /* 2. 超时扫描 */
    if (rsvr->ctm - rsvr->scan_tm > ACC_TMOUT_SCAN_SEC) {
        rsvr->scan_tm = rsvr->ctm;

        acc_rsvr_timeout_hdl(ctx, rsvr);
    }

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_get_timeout_conn_list
 **功    能: 将超时连接加入链表
 **输入参数: 
 **     node: 平衡二叉树结点
 **     timeout: 超时链表
 **输出参数: NONE
 **返    回: 代理对象
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.12.24 #
 ******************************************************************************/
static int acc_rsvr_get_timeout_conn_list(socket_t *sck, acc_conn_timeout_list_t *timeout)
{
#define ACC_SCK_TIMEOUT_SEC (180)

    /* 判断是否超时，则加入到timeout链表中 */
    if ((timeout->ctm - sck->rdtm <= ACC_SCK_TIMEOUT_SEC)
        || (timeout->ctm - sck->wrtm <= ACC_SCK_TIMEOUT_SEC))
    {
        return ACC_OK; /* 未超时 */
    }

    return list_lpush(timeout->list, sck);
}

/******************************************************************************
 **函数名称: acc_rsvr_conn_timeout
 **功    能: 删除超时连接
 **输入参数: 
 **     ctx: 全局信息
 **     rsvr: 接收服务
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2015.06.11 15:02:33 #
 ******************************************************************************/
static int acc_rsvr_conn_timeout(acc_cntx_t *ctx, acc_rsvr_t *rsvr)
{
    void *pool;
    socket_t *sck;
    list_opt_t opt;
    acc_conn_timeout_list_t timeout;

    memset(&timeout, 0, sizeof(timeout));

    /* > 创建内存池 */
    pool = mem_pool_creat(1 * KB);
    if (NULL == pool) {
        log_error(rsvr->log, "Create memory pool failed!");
        return ACC_ERR;
    }

    timeout.ctm = rsvr->ctm;

    do {
        /* > 创建链表 */
        memset(&opt, 0, sizeof(opt));

        opt.pool = pool;
        opt.alloc = (mem_alloc_cb_t)mem_pool_alloc;
        opt.dealloc = (mem_dealloc_cb_t)mem_pool_dealloc;

        timeout.list = list_creat(&opt);
        if (NULL == timeout.list) {
            log_error(rsvr->log, "Create list failed!");
            break;
        }

        /* > 获取超时连接 */
        spin_lock(&ctx->connections[rsvr->id].lock);

        rbt_trav(ctx->connections[rsvr->id].sids,
             (trav_cb_t)acc_rsvr_get_timeout_conn_list, (void *)&timeout);

        spin_unlock(&ctx->connections[rsvr->id].lock);

        log_debug(rsvr->log, "Timeout connections: %d!", timeout.list->num);

        /* > 删除超时连接 */
        for (;;) {
            sck = (socket_t *)list_lpop(timeout.list);
            if (NULL == sck) {
                break;
            }

            acc_rsvr_del_conn(ctx, rsvr, sck);
        }
    } while(0);

    /* > 释放内存空间 */
    mem_pool_destroy(pool);

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_timeout_hdl
 **功    能: 事件超时处理
 **输入参数: 
 **     rsvr: 接收服务
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **     不必依次释放超时链表各结点的空间，只需一次性释放内存池便可释放所有空间.
 **作    者: # Qifeng.zou # 2014.11.28 #
 ******************************************************************************/
static int acc_rsvr_timeout_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr)
{
    acc_rsvr_conn_timeout(ctx, rsvr);
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_add_conn
 **功    能: 添加新的连接
 **输入参数: 
 **     ctx: 全局信息
 **     rsvr: 接收服务
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.29 #
 ******************************************************************************/
static int acc_rsvr_add_conn(acc_cntx_t *ctx, acc_rsvr_t *rsvr)
{
#define AGT_RSVR_CONN_POP_NUM   (1024)
    int num, idx;
    time_t ctm = time(NULL);
    socket_t *sck;
    struct epoll_event ev;
    acc_socket_extra_t *extra;
    acc_add_sck_t *add[AGT_RSVR_CONN_POP_NUM];

    while (1) {
        num = MIN(queue_used(ctx->connq[rsvr->id]), AGT_RSVR_CONN_POP_NUM);
        if (0 == num) {
            return ACC_OK;
        }

        /* > 取数据 */
        num = queue_mpop(ctx->connq[rsvr->id], (void **)add, num);
        if (0 == num) {
            continue;
        }

        for (idx=0; idx<num; ++idx) {
            /* > 申请SCK空间 */
            sck = calloc(1, sizeof(socket_t));
            if (NULL == sck) {
                log_error(rsvr->log, "Alloc memory from slab failed! sid:%lu",
                        add[idx]->sid);
                CLOSE(add[idx]->fd);
                queue_dealloc(ctx->connq[rsvr->id], add[idx]);
                continue;
            }

            memset(sck, 0, sizeof(socket_t));

            /* > 创建SCK关联对象 */
            extra = calloc(1, sizeof(acc_socket_extra_t));
            if (NULL == extra) {
                log_error(rsvr->log, "Alloc memory from slab failed! sid:%lu",
                        add[idx]->sid);
                CLOSE(add[idx]->fd);
                FREE(sck);
                queue_dealloc(ctx->connq[rsvr->id], add[idx]);
                continue;
            }

            extra->aid = rsvr->id;
            extra->sid = add[idx]->sid;
            extra->send_list = list_creat(NULL);
            if (NULL == extra->send_list) {
                log_error(rsvr->log, "Alloc memory from slab failed! sid:%lu",
                          add[idx]->sid);
                CLOSE(add[idx]->fd);
                FREE(sck);
                FREE(extra);
                queue_dealloc(ctx->connq[rsvr->id], add[idx]);
                continue;
            }

            sck->extra = extra;

            /* > 设置SCK信息 */
            sck->fd = add[idx]->fd;
            sck->wrtm = sck->rdtm = ctm;/* 记录当前时间 */
            memcpy(&sck->crtm, &add[idx]->crtm, sizeof(add[idx]->crtm)); /* 创建时间 */

            sck->recv.phase = SOCK_PHASE_RECV_INIT;
            sck->recv_cb = (socket_recv_cb_t)acc_recv_data;   /* Recv回调函数 */
            sck->send_cb = (socket_send_cb_t)acc_send_data;   /* Send回调函数*/

            queue_dealloc(ctx->connq[rsvr->id], add[idx]);      /* 释放连接队列空间 */

            /* > 插入红黑树中(以序列号为主键) */
            if (acc_sid_item_add(ctx, extra->sid, sck)) {
                log_error(rsvr->log, "Insert into avl failed! fd:%d sid:%lu",
                          sck->fd, extra->sid);
                CLOSE(sck->fd);
                list_destroy(extra->send_list, (mem_dealloc_cb_t)mem_dealloc, NULL);
                FREE(sck->extra);
                FREE(sck);
                return ACC_ERR;
            }

            log_debug(rsvr->log, "Insert into avl success! fd:%d sid:%lu",
                      sck->fd, extra->sid);

            /* > 加入epoll监听(首先是接收客户端搜索请求, 所以设置EPOLLIN) */
            memset(&ev, 0, sizeof(ev));

            ev.data.ptr = sck;
            ev.events = EPOLLIN | EPOLLET; /* 边缘触发 */

            epoll_ctl(rsvr->epid, EPOLL_CTL_ADD, sck->fd, &ev);
            ++rsvr->conn_total;
        }
    }

    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_rsvr_del_conn
 **功    能: 删除指定套接字
 **输入参数:
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 依次释放套接字对象各成员的空间
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.12.06 #
 ******************************************************************************/
static int acc_rsvr_del_conn(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    acc_socket_extra_t *extra = sck->extra;

    log_trace(rsvr->log, "fd:%d sid:%ld", sck->fd, extra->sid);

    /* > 剔除SID对象 */
    acc_sid_item_del(ctx, extra->sid);

    /* > 释放套接字空间 */
    CLOSE(sck->fd);
    list_destroy(extra->send_list, (mem_dealloc_cb_t)mem_dealloc, NULL);
    if (sck->recv.addr) {
        mem_ref_decr(sck->recv.addr);
    }
    FREE(sck->extra);
    FREE(sck);

    --rsvr->conn_total;
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_recv_head
 **功    能: 接收报头
 **输入参数:
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.12.01 #
 ******************************************************************************/
static int acc_recv_head(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    void *addr;
    int n, left;
    mesg_header_t *head;
    socket_snap_t *recv = &sck->recv;

    addr = recv->addr;

    while (1) {
        /* 1. 计算剩余字节 */
        left = sizeof(mesg_header_t) - recv->off;

        /* 2. 接收报头数据 */
        n = read(sck->fd, addr + recv->off, left);
        if (n == left) {
            recv->off += n;
            break; /* 接收完毕 */
        }
        else if (n > 0) {
            recv->off += n;
            continue;
        }
        else if (0 == n || ECONNRESET == errno) {
            log_info(rsvr->log, "Client disconnected. errmsg:[%d] %s! fd:[%d] n:[%d/%d]",
                    errno, strerror(errno), sck->fd, n, left);
            return ACC_SCK_CLOSE;
        }
        else if ((n < 0) && (EAGAIN == errno)) {
            return ACC_SCK_AGAIN; /* 等待下次事件通知 */
        }
        else if (EINTR == errno) {
            continue; 
        }
        log_error(rsvr->log, "errmsg:[%d] %s. fd:[%d]", errno, strerror(errno), sck->fd);
        return ACC_ERR;
    }

    /* 3. 校验报头数据 */
    head = (mesg_header_t *)addr;
    MESG_HEAD_NTOH(head, head);
    head->nid = ACC_GET_NODE_ID(ctx);

    if (!MESG_CHKSUM_ISVALID(head)) {
        log_error(rsvr->log, "Check head failed! type:%d len:%d flag:%d chksum:[0x%X/0x%X]",
            head->type, head->length, head->flag, head->chksum, MSG_CHKSUM_VAL);
        return ACC_ERR;
    }

    log_trace(rsvr->log, "Recv head success! type:%d len:%d flag:%d chksum:[0x%X/0x%X]",
            head->type, head->length, head->flag, head->chksum, MSG_CHKSUM_VAL);

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_recv_body
 **功    能: 接收报体
 **输入参数:
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.12.02 #
 ******************************************************************************/
static int acc_recv_body(acc_rsvr_t *rsvr, socket_t *sck)
{
    int n, left;
    mesg_header_t *head;
    socket_snap_t *recv = &sck->recv;

    head  = (mesg_header_t *)recv->addr;

    /* 1. 接收报体 */
    while (1) {
        left = recv->total - recv->off;

        n = read(sck->fd, recv->addr + recv->off, left);
        if (n == left) {
            recv->off += n;
            break; /* 接收完毕 */
        }
        else if (n > 0) {
            recv->off += n;
            continue;
        }
        else if (0 == n) {
            log_info(rsvr->log, "Client disconnected. errmsg:[%d] %s! fd:[%d] n:[%d/%d]",
                    errno, strerror(errno), sck->fd, n, left);
            return ACC_SCK_CLOSE;
        }
        else if ((n < 0) && (EAGAIN == errno)) {
            return ACC_SCK_AGAIN;
        }

        if (EINTR == errno) {
            continue;
        }

        log_error(rsvr->log, "errmsg:[%d] %s! fd:%d type:%d length:%d n:%d total:%d offset:%d addr:%p",
                errno, strerror(errno), head->type,
                sck->fd, head->length, n, recv->total, recv->off, recv->addr);
        return ACC_ERR;
    }

    log_trace(rsvr->log, "Recv body success! fd:%d type:%d length:%d total:%d off:%d",
            sck->fd, head->type, head->length, recv->total, recv->off);

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_sys_msg_hdl
 **功    能: 系统消息的处理
 **输入参数:
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2015.05.28 #
 ******************************************************************************/
static int acc_sys_msg_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_cmd_proc_req
 **功    能: 发送处理请求
 **输入参数:
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **     wid: 工作线程ID(与rqid一致)
 **输出参数: NONE
 **返    回: >0:成功 <=0:失败
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2015.06.26 10:33:12 #
 ******************************************************************************/
static int acc_rsvr_cmd_proc_req(acc_cntx_t *ctx, acc_rsvr_t *rsvr, int widx)
{
    cmd_data_t cmd;
    char path[FILE_PATH_MAX_LEN];

    memset(&cmd, 0, sizeof(cmd));

    cmd.type = CMD_PROC_DATA;

    acc_wsvr_cmd_usck_path(ctx->conf, widx, path, sizeof(path));

    /* > 发送处理命令 */
    return unix_udp_send(rsvr->cmd_sck.fd, path, &cmd, sizeof(cmd_data_t));
}

/******************************************************************************
 **函数名称: acc_recv_post_hdl
 **功    能: 数据接收完毕，进行数据处理
 **输入参数:
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项:
 **作    者: # Qifeng.zou # 2014.12.21 #
 ******************************************************************************/
static int acc_recv_post_hdl(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    acc_reg_t *reg;
    mesg_header_t *head;
    acc_socket_extra_t *extra = (acc_socket_extra_t *)sck->extra;

    /* > 自定义消息的处理 */
    if (MSG_FLAG_USR == extra->head->flag) {
        head = (mesg_header_t *)extra->head;

        reg = &ctx->reg[head->type];

        /* > 调用处理回调 */
        reg->proc(head->type, (void *)(head + 1),
                head->length + sizeof(mesg_header_t), reg->args);

        /* 3. 释放内存空间 */
        mem_ref_decr((void *)head);
        return ACC_OK;
    }

    /* > 系统消息的处理 */
    return acc_sys_msg_hdl(ctx, rsvr, sck);
}

/******************************************************************************
 **函数名称: acc_recv_data
 **功    能: 接收数据
 **输入参数:
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: TODO: 此处理流程可进一步进行优化
 **作    者: # Qifeng.zou # 2014.11.29 #
 ******************************************************************************/
static int acc_recv_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    int ret;
    mesg_header_t *head;
    socket_snap_t *recv = &sck->recv;
    queue_conf_t *conf = &ctx->conf->recvq;
    acc_socket_extra_t *extra = (acc_socket_extra_t *)sck->extra;

    for (;;) {
        switch (recv->phase) {
            case SOCK_PHASE_RECV_INIT: /* 1. 分配空间 */
                recv->addr = mem_ref_alloc(conf->size,
                        NULL, (mem_alloc_cb_t)mem_alloc, (mem_dealloc_cb_t)mem_dealloc);
                if (NULL == recv->addr) {
                    log_error(rsvr->log, "Alloc from queue failed!");
                    return ACC_ERR;
                }

                log_info(rsvr->log, "Alloc memory from queue success!");

                extra->head = (mesg_header_t *)recv->addr;
                extra->body = (void *)(extra->head + 1);
                recv->off = 0;
                recv->total = sizeof(mesg_header_t);

                /* 设置下步 */
                recv->phase = SOCK_PHASE_RECV_HEAD;

                goto RECV_HEAD;
            case SOCK_PHASE_RECV_HEAD: /* 2. 接收报头 */
            RECV_HEAD:
                ret = acc_recv_head(ctx, rsvr, sck);
                switch (ret) {
                    case ACC_OK:
                        head = (mesg_header_t *)recv->addr;
                        head->sid = extra->sid;
                        head->serial = tlz_gen_serail( /* 获取流水号 */
                                ctx->conf->nid, rsvr->id, ++rsvr->recv_seq);

                        log_info(rsvr->log, "serial:%lu", head->serial);

                        if (head->length) {
                            recv->phase = SOCK_PHASE_READY_BODY; /* 设置下步 */
                        }
                        else {
                            recv->phase = SOCK_PHASE_RECV_POST; /* 设置下步 */
                            goto RECV_POST;
                        }
                        break;      /* 继续后续处理 */
                    case ACC_SCK_AGAIN:
                        return ret; /* 下次继续处理 */
                    default:
                        mem_ref_decr(recv->addr);
                        recv->addr = NULL;
                        return ret; /* 异常情况 */
                }
                goto READY_BODY;
            case SOCK_PHASE_READY_BODY: /* 3. 准备接收报体 */
            READY_BODY:
                recv->total += extra->head->length;

                /* 设置下步 */
                recv->phase = SOCK_PHASE_RECV_BODY;

                goto RECV_BODY;
            case SOCK_PHASE_RECV_BODY: /* 4. 接收报体 */
            RECV_BODY:
                ret = acc_recv_body(rsvr, sck);
                switch (ret) {
                    case ACC_OK:
                        recv->phase = SOCK_PHASE_RECV_POST; /* 设置下步 */
                        break;      /* 继续后续处理 */
                    case ACC_SCK_AGAIN:
                        return ret; /* 下次继续处理 */
                    default:
                        mem_ref_decr(recv->addr);
                        recv->addr = NULL;
                        return ret; /* 异常情况 */
                }
                goto RECV_POST;
            case SOCK_PHASE_RECV_POST: /* 5. 接收完毕: 数据处理 */
            RECV_POST:
                /* 将数据放入接收队列 */
                ret = acc_recv_post_hdl(ctx, rsvr, sck);
                if (ACC_OK == ret) {
                        recv->phase = SOCK_PHASE_RECV_INIT;
                        recv->addr = NULL;
                        continue; /* 接收下一条数据 */
                }
                mem_ref_decr(recv->addr);
                recv->addr = NULL;
                return ACC_ERR;
        }
    }

    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_send_data
 **功    能: 发送数据
 **输入参数:
 **     rsvr: 接收服务
 **     sck: SCK对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.29 #
 ******************************************************************************/
static int acc_send_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr, socket_t *sck)
{
    int n, left;
    mesg_header_t *head, hhead;
    struct epoll_event ev;
    socket_snap_t *send = &sck->send;
    acc_socket_extra_t *extra = (acc_socket_extra_t *)sck->extra;

    sck->wrtm = time(NULL);

    for (;;) {
        /* 1. 取发送的数据 */
        if (NULL == send->addr) {
            send->addr = list_lpop(extra->send_list);
            if (NULL == send->addr) {
                return ACC_OK; /* 无数据 */
            }

            head = (mesg_header_t *)send->addr;

            MESG_HEAD_NTOH(head, &hhead);
            MESG_HEAD_PRINT(rsvr->log, &hhead);

            send->off = 0;
            send->total = hhead.length + sizeof(mesg_header_t);

            log_trace(rsvr->log, "sid:%lu serial:%lu!", hhead.sid, hhead.serial);
        }

        /* 2. 发送数据 */
        left = send->total - send->off;

        n = Writen(sck->fd, send->addr+send->off, left);
        if (n != left) {
            if (n > 0) {
                send->off += n;
                return ACC_SCK_AGAIN;
            }

            log_error(rsvr->log, "errmsg:[%d] %s!", errno, strerror(errno));

            /* 释放空间 */
            FREE(send->addr);
            send->addr = NULL;
            return ACC_ERR;
        }

        /* 3. 释放空间 */
        FREE(send->addr);
        send->addr = NULL;

        /* > 设置epoll监听 */
        memset(&ev, 0, sizeof(ev));

        ev.data.ptr = sck;
        ev.events = list_empty(extra->send_list)?
            (EPOLLIN|EPOLLET) : (EPOLLIN|EPOLLOUT|EPOLLET); /* 边缘触发 */

        epoll_ctl(rsvr->epid, EPOLL_CTL_MOD, sck->fd, &ev);
    }

    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_rsvr_dist_send_data
 **功    能: 分发发送的数据
 **输入参数:
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 千万勿将共享变量参与MIN()三目运算, 否则可能出现严重错误!!!!且很难找出原因!
 **作    者: # Qifeng.zou # 2015-06-05 17:35:02 #
 ******************************************************************************/
static int acc_rsvr_dist_send_data(acc_cntx_t *ctx, acc_rsvr_t *rsvr)
{
    int num, idx;
    ring_t *sendq;
    socket_t *sck;
    mesg_header_t *head, hhead;
    struct epoll_event ev;
    void *addr[AGT_RSVR_DIST_POP_NUM];

    sendq = ctx->sendq[rsvr->id];
    while (1) {
        num = MIN(ring_used(sendq), AGT_RSVR_DIST_POP_NUM);
        if (0 == num) {
            break;
        }

        /* > 弹出应答数据 */
        num = ring_mpop(sendq, addr, num);
        if (0 == num) {
            break;
        }

        log_debug(rsvr->log, "Pop data succ! num:%d", num);

        for (idx=0; idx<num; ++idx) {
            head = (mesg_header_t *)addr[idx];  // 消息头

            MESG_HEAD_NTOH(head, &hhead);       // 字节序转换
            if (!MESG_CHKSUM_ISVALID(&hhead)) {
                log_error(ctx->log, "Check chksum [0x%X/0x%X] failed! sid:%lu serial:%lu",
                        hhead.chksum, MSG_CHKSUM_VAL, hhead.sid, hhead.serial);
                FREE(addr[idx]);
                continue;
            }

            /* > 发入发送列表 */
            sck = acc_push_into_send_list(ctx, rsvr, hhead.sid, addr[idx]); 
            if (NULL == sck) {
                log_error(ctx->log, "Query socket failed! serial:%lu sid:%lu",
                        hhead.serial, hhead.sid);
                FREE(addr[idx]);
                continue;
            }

            /* > 设置epoll监听(添加EPOLLOUT) */
            memset(&ev, 0, sizeof(ev));

            ev.data.ptr = sck;
            ev.events = EPOLLOUT | EPOLLET; /* 边缘触发 */

            epoll_ctl(rsvr->epid, EPOLL_CTL_MOD, sck->fd, &ev);
        }
    }

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_push_into_send_list
 **功    能: 将数据放入发送列表
 **输入参数:
 **     ctx: 全局对象
 **     rsvr: 接收服务
 **     sid: 会话ID
 **     addr: 需要发送的数据
 **输出参数: NONE
 **返    回: 连接对象
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2016-07-24 22:49:02 #
 ******************************************************************************/
static socket_t *acc_push_into_send_list(
        acc_cntx_t *ctx, acc_rsvr_t *rsvr, uint64_t sid, void *addr)
{
    socket_t *sck, key;
    acc_sid_list_t *list;
    acc_socket_extra_t *extra, key_extra;

    key_extra.sid = sid;
    key.extra = &key_extra;

    list = &ctx->connections[rsvr->id];

    /* > 查询会话对象 */
    spin_lock(&list->lock);

    sck = rbt_query(list->sids, &key);
    if (NULL == sck) {
        spin_unlock(&list->lock);
        return NULL;
    }

    extra = (acc_socket_extra_t *)sck->extra;

    /* > 放入发送列表 */
    if (list_rpush(extra->send_list, addr)) {
        spin_unlock(&list->lock);
        return NULL;
    }

    spin_unlock(&list->lock);

    return sck;
}
