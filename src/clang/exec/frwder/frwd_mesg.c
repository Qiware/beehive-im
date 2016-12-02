/******************************************************************************
 ** Coypright(C) 2014-2024 Qiware technology Co., Ltd
 **
 ** 文件名: frwd_mesg.c
 ** 版本号: 1.0
 ** 描  述: 消息处理函数定义
 ** 作  者: # Qifeng.zou # Tue 14 Jul 2015 02:52:16 PM CST #
 ******************************************************************************/
#include "mesg.h"
#include "frwder.h"
#include "vector.h"
#include "cmd_list.h"

/* 静态函数 */
static int frwd_mesg_from_fw_def_hdl(int type, int orig, char *data, size_t len, void *args);
static int frwd_mesg_from_bc_def_hdl(int type, int orig, char *data, size_t len, void *args);

/******************************************************************************
 **函数名称: frwd_set_reg
 **功    能: 注册处理回调
 **输入参数:
 **     frwd: 全局对象
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2015.06.10 #
 ******************************************************************************/
int frwd_set_reg(frwd_cntx_t *frwd)
{
#define FRWD_REG_REQ_CB(frwd, type, proc, args) \
    if (rtmq_register((frwd)->forward, type, (rtmq_reg_cb_t)proc, (void *)args)) { \
        log_error((frwd)->log, "Register type [%d] failed!", type); \
        return FRWD_ERR; \
    }

    FRWD_REG_REQ_CB(frwd, CMD_UNKNOWN, frwd_mesg_from_fw_def_hdl, frwd);

#define FRWD_REG_RSP_CB(frwd, type, proc, args) \
    if (rtmq_register((frwd)->backend, type, (rtmq_reg_cb_t)proc, (void *)args)) { \
        log_error((frwd)->log, "Register type [%d] failed!", type); \
        return FRWD_ERR; \
    }

    FRWD_REG_RSP_CB(frwd, CMD_UNKNOWN, frwd_mesg_from_bc_def_hdl, frwd);

    return FRWD_OK;
}

/******************************************************************************
 **函数名称: frwd_mesg_from_fw_def_hdl
 **功    能: 来自下游结点的消息
 **输入参数:
 **     type: 数据类型
 **     orig: 源结点ID
 **     data: 需要转发的数据
 **     len: 数据长度
 **     args: 附加参数
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.09.30 15:26:44 #
 ******************************************************************************/
static int frwd_mesg_from_fw_def_hdl(int type, int orig, char *data, size_t len, void *args)
{
    int nid;
    frwd_cntx_t *ctx = (frwd_cntx_t *)args;
    mesg_header_t *head = (mesg_header_t *)data;

    /* > 转换字节序 */
    MESG_HEAD_NTOH(head, head);

    log_trace(ctx->log, "sid:%lu serial:%lu type:%d len:%d flag:%d chksum:[0x%X/0x%X]",
            head->sid, head->serial, head->type,
            head->length, head->flag, head->chksum, MSG_CHKSUM_VAL);

    MESG_HEAD_HTON(head, head);

    /* > 发送数据 */
    nid = rtmq_sub_query(ctx->backend, type);
    if (-1 == nid) {
        log_error(ctx->log, "No module sub type! type:0x%04X", type);
        return 0;
    }

    return rtmq_async_send(ctx->backend, type, nid, data, len);
}

/******************************************************************************
 **函数名称: frwd_mesg_from_bc_def_hdl
 **功    能: 来自上游结点的消息
 **输入参数:
 **     type: 数据类型
 **     orig: 源结点ID
 **     data: 需要转发的数据
 **     len: 数据长度
 **     args: 附加参数
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.09.30 15:26:44 #
 ******************************************************************************/
static int frwd_mesg_from_bc_def_hdl(int type, int orig, char *data, size_t len, void *args)
{
    serial_t serial;
    frwd_cntx_t *ctx = (frwd_cntx_t *)args;
    mesg_header_t *head = (mesg_header_t *)data;

    MESG_HEAD_NTOH(head, head);

    serial.serial = head->serial;
    log_trace(ctx->log, "serial:%lu", head->serial);

    MESG_HEAD_HTON(head, head);

    /* > 发送数据 */
    return rtmq_async_send(ctx->forward, type, serial.nid, data, len);
}
