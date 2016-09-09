#if !defined(__COMMAND_H__)
#define __COMMAND_H__

/* 命令类型 */
typedef enum
{
    CMD_TYPE_UNKOWN = 0x0000                /* 未知类型 */
    , CMD_ADD_SCK   = 0x0001                /* 添加套接字 */
    , CMD_DIST_DATA = 0x0002                /* 分发数据 */
    , CMD_PROC_DATA = 0x0003                /* 处理数据 */
} cmd_type_e;

/* 分发数据参数 */
typedef struct
{
    int qid;                                /* 队列ID */
} cmd_dist_data_t;

/* 命令附加信息 */
typedef union
{
    cmd_dist_data_t dist_data;              /* 分发数据 */
} cmd_param_u;

/* 命令数据信息 */
typedef struct
{
    unsigned int type;                      /* 命令类型(cmd_type_e) */
    char path[FILE_NAME_MAX_LEN];           /* 命令源路径 */
    cmd_param_u param;                      /* 附加数据信息 */
} cmd_data_t;

#endif /*__COMMAND_H__*/
