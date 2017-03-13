/******************************************************************************
 ** Coypright(C) 2014-2024 Qiware technology Co., Ltd
 **
 ** 文件名: iovec.h
 ** 版本号: 1.0
 ** 描  述: 
 ** 作  者: # Qifeng.zou # 2015年12月26日 星期六 08时11分09秒 #
 ******************************************************************************/

#if !defined(__IO_VEC_H__)
#define __IO_VEC_H__

#include "comm.h"

typedef int (*wiov_send_succ_cb_t)(void *ctx, void *addr, size_t len);
typedef int (*wiov_send_fail_cb_t)(void *ctx, void *addr, size_t len);

/* IOV原始数据信息 */
typedef struct
{
    void *addr;                         /* 起始地址 */
    size_t len;                         /* 原始长度 */

    void *param;                        /* 附加参数 */
    wiov_send_succ_cb_t succ;           /* 发送成功处理回调 */
    wiov_send_fail_cb_t fail;           /* 发送失败处理回调 */
} wiov_orig_t;

/* IOV对象(写) */
typedef struct
{
    int iov_max;                        /* 最大空间限制 */
    int iov_cnt;                        /* 发送缓存个数 */
    int iov_idx;                        /* 当前正在发送的缓存索引 */
    wiov_orig_t *orig;                  /* 原始信息(注: 与iov[]一一对应) */
    struct iovec *iov;                  /* 发送缓存 */
} wiov_t;

int wiov_init(wiov_t *wiov, int max);
void wiov_destroy(wiov_t *wiov);

#define wiov_isempty(wiov) (0 == (wiov)->iov_cnt) // 缓存已空
#define wiov_isfull(wiov) ((wiov)->iov_max == (wiov)->iov_cnt) // 缓存已满
#define wiov_left_space(wiov) ((wiov)->iov_max - (wiov)->iov_cnt) // 剩余空间

#define wiov_item_begin(wiov) ((wiov)->iov + (wiov)->iov_idx)
#define wiov_item_num(wiov) ((wiov)->iov_cnt - (wiov)->iov_idx)

/* 添加发送内容 */
#define wiov_item_add(wiov, _addr, _len, _param, _succ, _fail) \
{ \
    (wiov)->iov[(wiov)->iov_cnt].iov_len = (_len); \
    (wiov)->iov[(wiov)->iov_cnt].iov_base = (char *)(_addr); \
    \
    (wiov)->orig[(wiov)->iov_cnt].len = (_len); \
    (wiov)->orig[(wiov)->iov_cnt].addr = (char *)(_addr); \
    \
    (wiov)->orig[(wiov)->iov_cnt].param = (void *)(_param); \
    (wiov)->orig[(wiov)->iov_cnt].succ = (wiov_send_succ_cb_t)(_succ); \
    (wiov)->orig[(wiov)->iov_cnt].fail = (wiov_send_fail_cb_t)(_fail); \
    \
    ++(wiov)->iov_cnt; \
}

/* 重置item项 */
#define wiov_item_reset(wiov, idx) \
{ \
    (wiov)->iov[idx].iov_len = 0; \
    (wiov)->iov[idx].iov_base = NULL; \
    \
    (wiov)->orig[idx].len = 0; \
    (wiov)->orig[idx].addr = NULL; \
    (wiov)->orig[idx].param = NULL; \
    (wiov)->orig[idx].succ = NULL; \
    (wiov)->orig[idx].fail = NULL; \
    \
}

void wiov_clean(wiov_t *wiov);
int wiov_item_adjust(wiov_t *wiov, size_t n);

#endif /*__IO_VEC_H__*/
