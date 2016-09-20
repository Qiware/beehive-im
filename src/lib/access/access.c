#include "log.h"
#include "sck.h"
#include "comm.h"
#include "lock.h"
#include "redo.h"
#include "search.h"
#include "acc_lsn.h"
#include "acc_rsvr.h"
#include "hash_alg.h"

static int acc_comm_init(acc_cntx_t *ctx);
static int acc_creat_rsvr(acc_cntx_t *ctx);
static int acc_rsvr_pool_destroy(acc_cntx_t *ctx);
static int acc_creat_lsvr(acc_cntx_t *ctx);
static int acc_creat_queue(acc_cntx_t *ctx);

/* CID比较回调 */
static int acc_conn_cid_cmp_cb(const socket_t *sck1, const socket_t *sck2)
{
    acc_socket_extra_t *extra1, *extra2;

    extra1 = (acc_socket_extra_t *)sck1->extra;
    extra2 = (acc_socket_extra_t *)sck2->extra;

    return (extra1->cid - extra2->cid);
}

/* CID哈希回调 */
static int acc_conn_cid_hash_cb(const socket_t *sck)
{
    return ((acc_socket_extra_t *)sck->extra)->cid;
}

/******************************************************************************
 **函数名称: acc_init
 **功    能: 初始化全局信息
 **输入参数: 
 **     protocol: 协议
 **     conf: 配置路径
 **     log: 日志对象
 **输出参数: NONE
 **返    回: 全局对象
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.15 #
 ******************************************************************************/
acc_cntx_t *acc_init(acc_protocol_t *protocol, acc_conf_t *conf, log_cycle_t *log)
{
    acc_cntx_t *ctx;

    /* > 创建全局对象 */
    ctx = (acc_cntx_t *)calloc(1, sizeof(acc_cntx_t));
    if (NULL == ctx) {
        log_error(log, "errmsg:[%d] %s!", errno, strerror(errno));
        return NULL;
    }

    ctx->log = log;
    ctx->conf = conf;
    ctx->protocol = protocol;

    do {
        /* > 设置进程打开文件数 */
        if (set_fd_limit(conf->connections.max)) {
            log_error(log, "errmsg:[%d] %s! max:%d",
                      errno, strerror(errno), conf->connections.max);
            break;
        }

        /* > 创建队列 */
        if (acc_creat_queue(ctx)) {
            log_error(log, "errmsg:[%d] %s!", errno, strerror(errno));
            break;
        }

        /* > 创建Agent线程池 */
        if (acc_creat_rsvr(ctx)) {
            log_error(log, "Initialize agent thread pool failed!");
            break;
        }

        /* > 创建Listen线程池 */
        if (acc_creat_lsvr(ctx)) {
            log_error(log, "Initialize agent thread pool failed!");
            break;
        }

        /* > 创建连接管理 */
        ctx->conn_cid_tab = (hash_tab_t *)hash_tab_creat(conf->rsvr_num,
            (hash_cb_t)acc_conn_cid_hash_cb, (cmp_cb_t)acc_conn_cid_cmp_cb, NULL);
        if (NULL == ctx->conn_cid_tab) {
            log_error(ctx->log, "Init sid list failed!");
            break;
        }

        /* > 初始化其他信息 */
        if (acc_comm_init(ctx)) {
            log_error(log, "Initialize client failed!");
            break;
        }

        return ctx;
    } while (0);

    free(ctx);
    return NULL;
}

/******************************************************************************
 **函数名称: acc_destroy
 **功    能: 销毁代理服务上下文
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 依次销毁侦听线程、接收线程、工作线程、日志对象等
 **注意事项: 按序销毁
 **作    者: # Qifeng.zou # 2014.11.17 #
 ******************************************************************************/
void acc_destroy(acc_cntx_t *ctx)
{
    acc_rsvr_pool_destroy(ctx);
}

/******************************************************************************
 **函数名称: acc_launch
 **功    能: 启动代理服务
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **     设置线程回调
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.15 #
 ******************************************************************************/
int acc_launch(acc_cntx_t *ctx)
{
    int idx;
    acc_conf_t *conf = ctx->conf;

    /* 1. 设置Agent线程回调 */
    for (idx=0; idx<conf->rsvr_num; ++idx) {
        thread_pool_add_worker(ctx->rsvr_pool, acc_rsvr_routine, ctx);
    }
    
    /* 2. 设置Listen线程回调 */
    for (idx=0; idx<conf->lsvr_num; ++idx) {
        thread_pool_add_worker(ctx->lsvr_pool, acc_lsvr_routine, ctx);
    }
 
    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_creat_rsvr
 **功    能: 创建接收线程池
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.15 #
 ******************************************************************************/
static int acc_creat_rsvr(acc_cntx_t *ctx)
{
    int idx, num;
    acc_rsvr_t *agent;
    const acc_conf_t *conf = ctx->conf;

    /* > 新建Agent对象 */
    agent = (acc_rsvr_t *)calloc(1, conf->rsvr_num*sizeof(acc_rsvr_t));
    if (NULL == agent) {
        log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
        return ACC_ERR;
    }

    /* > 创建Worker线程池 */
    ctx->rsvr_pool = thread_pool_init(conf->rsvr_num, NULL, agent);
    if (NULL == ctx->rsvr_pool) {
        log_error(ctx->log, "Initialize thread pool failed!");
        free(agent);
        return ACC_ERR;
    }

    /* 3. 依次初始化Agent对象 */
    for (idx=0; idx<conf->rsvr_num; ++idx) {
        if (acc_rsvr_init(ctx, agent+idx, idx)) {
            log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
            break;
        }
    }

    if (idx == conf->rsvr_num) {
        return ACC_OK; /* 成功 */
    }

    /* 4. 释放Agent对象 */
    num = idx;
    for (idx=0; idx<num; ++idx) {
        acc_rsvr_destroy(agent+idx);
    }

    FREE(agent);
    thread_pool_destroy(ctx->rsvr_pool);

    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_creat_lsvr
 **功    能: 创建帧听线程池
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2015-06-30 15:06:58 #
 ******************************************************************************/
static int acc_creat_lsvr(acc_cntx_t *ctx)
{
    int idx;
    acc_lsvr_t *lsvr;
    acc_conf_t *conf = ctx->conf;

    /* > 侦听指定端口 */
    ctx->listen.lsn_sck_id = tcp_listen(conf->connections.port);
    if (ctx->listen.lsn_sck_id < 0) {
        log_error(ctx->log, "errmsg:[%d] %s! port:%d",
                  errno, strerror(errno), conf->connections.port);
        return ACC_ERR;
    }

    spin_lock_init(&ctx->listen.accept_lock);

    /* > 创建LSN对象 */
    ctx->listen.lsvr = (acc_lsvr_t *)calloc(1, conf->lsvr_num*sizeof(acc_lsvr_t));
    if (NULL == ctx->listen.lsvr) {
        CLOSE(ctx->listen.lsn_sck_id);
        log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
        return ACC_ERR;
    }

    /* > 初始化侦听服务 */
    for (idx=0; idx<conf->lsvr_num; ++idx) {
        lsvr = ctx->listen.lsvr + idx;
        lsvr->log = ctx->log;
        if (acc_lsvr_init(ctx, lsvr, idx)) {
            CLOSE(ctx->listen.lsn_sck_id);
            FREE(ctx->listen.lsvr);
            log_error(ctx->log, "Initialize listen-server failed!");
            return ACC_ERR;
        }
    }

    ctx->lsvr_pool = thread_pool_init(conf->lsvr_num, NULL, ctx->listen.lsvr);
    if (NULL == ctx->lsvr_pool) {
        CLOSE(ctx->listen.lsn_sck_id);
        FREE(ctx->listen.lsvr);
        log_error(ctx->log, "Initialize thread pool failed!");
        return ACC_ERR;
    }

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_rsvr_pool_destroy
 **功    能: 销毁Agent线程池
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.11.15 #
 ******************************************************************************/
static int acc_rsvr_pool_destroy(acc_cntx_t *ctx)
{
    int idx;
    void *data;
    acc_rsvr_t *agent;
    const acc_conf_t *conf = ctx->conf;

    /* 1. 释放Agent对象 */
    for (idx=0; idx<conf->rsvr_num; ++idx) {
        agent = (acc_rsvr_t *)ctx->rsvr_pool->data + idx;

        acc_rsvr_destroy(agent);
    }

    /* 2. 释放线程池对象 */
    data = ctx->rsvr_pool->data;

    thread_pool_destroy(ctx->rsvr_pool);

    free(data);

    ctx->rsvr_pool = NULL;

    return ACC_ERR;
}

/******************************************************************************
 **函数名称: acc_reg_def_hdl
 **功    能: 默认注册函数
 **输入参数:
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 
 **作    者: # Qifeng.zou # 2014.12.20 #
 ******************************************************************************/
static int acc_reg_def_hdl(unsigned int type, char *buff, size_t len, void *args)
{
    static int total = 0;
    acc_cntx_t *ctx = (acc_cntx_t *)args;

    log_info(ctx->log, "total:%d", ++total);

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_creat_queue
 **功    能: 创建队列
 **输入参数:
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述: 
 **注意事项: 此过程一旦失败, 程序必须退出运行. 因此, 在此申请的内存未被主动释放也不算内存泄露!
 **作    者: # Qifeng.zou # 2014.12.21 #
 ******************************************************************************/
static int acc_creat_queue(acc_cntx_t *ctx)
{
    int idx;
    const acc_conf_t *conf = ctx->conf;

    /* > 创建CONN队列(与Agent数一致) */
    ctx->connq = (queue_t **)calloc(conf->rsvr_num, sizeof(queue_t*));
    if (NULL == ctx->connq) {
        log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
        return ACC_ERR;
    }

    for (idx=0; idx<conf->rsvr_num; ++idx) {
        ctx->connq[idx] = queue_creat(conf->connq.max, sizeof(acc_add_sck_t));
        if (NULL == ctx->connq[idx]) {
            log_error(ctx->log, "Create conn queue failed!");
            return ACC_ERR;
        }
    }

    /* > 创建RECV队列(与Agent数一致) */
    ctx->recvq = (ring_t **)calloc(conf->rsvr_num, sizeof(ring_t*));
    if (NULL == ctx->recvq) {
        log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
        return ACC_ERR;
    }

    for (idx=0; idx<conf->rsvr_num; ++idx) {
        ctx->recvq[idx] = ring_creat(conf->recvq.max);
        if (NULL == ctx->recvq[idx]) {
            log_error(ctx->log, "Create recv queue failed!");
            return ACC_ERR;
        }
    }

    /* > 创建SEND队列(与Agent数一致) */
    ctx->sendq = (ring_t **)calloc(conf->rsvr_num, sizeof(ring_t *));
    if (NULL == ctx->sendq) {
        log_error(ctx->log, "errmsg:[%d] %s!", errno, strerror(errno));
        return ACC_ERR;
    }

    for (idx=0; idx<conf->rsvr_num; ++idx) {
        ctx->sendq[idx] = ring_creat(conf->sendq.max);
        if (NULL == ctx->sendq[idx]) {
            log_error(ctx->log, "Create send queue failed!");
            return ACC_ERR;
        }
    }

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_comm_init
 **功    能: 初始化通用信息
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项: 
 **作    者: # Qifeng.zou # 2015-06-24 23:58:46 #
 ******************************************************************************/
static int acc_comm_init(acc_cntx_t *ctx)
{
    char path[FILE_PATH_MAX_LEN];

    snprintf(path, sizeof(path), "%s/"ACC_CLI_CMD_PATH, ctx->conf->path);

    ctx->cmd_sck_id = unix_udp_creat(path);
    if (ctx->cmd_sck_id < 0) {
        return ACC_ERR;
    }

    return ACC_OK;
}

/******************************************************************************
 **函数名称: acc_conn_cid_tab_add
 **功    能: 新增CID列表
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项: 
 **作    者: # Qifeng.zou # 2015-06-24 23:58:46 #
 ******************************************************************************/
int acc_conn_cid_tab_add(acc_cntx_t *ctx, socket_t *sck)
{
    return hash_tab_insert(ctx->conn_cid_tab, sck, WRLOCK);
}

/******************************************************************************
 **函数名称: acc_conn_cid_tab_del
 **功    能: 删除CID列表
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项: 
 **作    者: # Qifeng.zou # 2015-06-24 23:58:46 #
 ******************************************************************************/
socket_t *acc_conn_cid_tab_del(acc_cntx_t *ctx, uint64_t cid)
{
    socket_t key;
    acc_socket_extra_t extra;

    extra.cid = cid;
    key.extra = &extra;

    return hash_tab_delete(ctx->conn_cid_tab, &key, WRLOCK);
}

/******************************************************************************
 **函数名称: acc_get_rid_by_cid
 **功    能: 通过cid查找rsvr的序列号
 **输入参数: 
 **     ctx: 全局信息
 **输出参数: NONE
 **返    回: RSVR的序列号
 **实现描述:
 **注意事项: 
 **作    者: # Qifeng.zou # 2015-06-24 23:58:46 #
 ******************************************************************************/
int acc_get_rid_by_cid(acc_cntx_t *ctx, uint64_t cid)
{
    int rid;
    socket_t *sck, key;
    acc_socket_extra_t *extra, key_extra;

    key_extra.cid = cid;
    key.extra = &key_extra;

    sck = hash_tab_query(ctx->conn_cid_tab, &key, RDLOCK);
    if (NULL == sck) {
        return -1;
    }

    extra = (acc_socket_extra_t *)sck->extra;
    rid = extra->rid;

    hash_tab_unlock(ctx->conn_cid_tab, &key, RDLOCK);

    return rid;
}
