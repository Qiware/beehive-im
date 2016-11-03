package ctrl

import (
	"errors"
	"fmt"
	"os"

	"github.com/astaxie/beego/logs"
	"github.com/garyburd/redigo/redis"

	"chat/src/golang/lib/comm"
	"chat/src/golang/lib/rtmq"
)

/* OLS上下文 */
type MsgSvrCntx struct {
	conf  *MsgSvrConf         /* 配置信息 */
	log   *logs.BeeLogger     /* 日志对象 */
	proxy *rtmq.RtmqProxyCntx /* 代理对象 */
	redis *redis.Pool         /* REDIS连接池 */
}

/******************************************************************************
 **函数名称: MsgSvrInit
 **功    能: 初始化对象
 **输入参数:
 **     conf: 配置信息
 **输出参数: NONE
 **返    回:
 **     ctx: 上下文
 **     err: 错误信息
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func MsgSvrInit(conf *MsgSvrConf) (ctx *MsgSvrCntx, err error) {
	ctx = &MsgSvrCntx{}

	ctx.conf = conf

	/* > 初始化日志 */
	if err := ctx.log_init(); nil != err {
		return nil, err
	}

	/* > REDIS连接池 */
	ctx.redis = &redis.Pool{
		MaxIdle:   80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", conf.RedisAddr)
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
	if nil == ctx.redis {
		ctx.log.Error("Create redis pool failed! addr:%s", conf.RedisAddr)
		return nil, errors.New("Create redis pool failed!")
	}

	/* > 初始化RTMQ-PROXY */
	ctx.proxy = rtmq.ProxyInit(&conf.proxy, ctx.log)
	if nil == ctx.proxy {
		return nil, err
	}

	return ctx, nil
}

/******************************************************************************
 **函数名称: Register
 **功    能: 注册处理回调
 **输入参数: NONE
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 注册回调函数
 **注意事项: 请在调用Launch()前完成此函数调用
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func (ctx *MsgSvrCntx) Register() {
	ctx.proxy.Register(comm.CMD_GROUP_MSG, MsgSvrGroupMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_GROUP_MSG_ACK, MsgSvrGroupMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_PRVT_MSG, MsgSvrPrvtMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_PRVT_MSG_ACK, MsgSvrPrvtMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_BC_MSG, MsgSvrBcMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_BC_MSG_ACK, MsgSvrBcMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_P2P_MSG, MsgSvrP2pMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_P2P_MSG_ACK, MsgSvrP2pMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_ROOM_MSG, MsgSvrRoomMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_ROOM_MSG_ACK, MsgSvrRoomMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_ROOM_BC_MSG, MsgSvrRoomBcMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_ROOM_BC_MSG_ACK, MsgSvrRoomBcMsgAckHandler, ctx)

	ctx.proxy.Register(comm.CMD_SYNC_MSG, MsgSvrSyncMsgHandler, ctx)
	ctx.proxy.Register(comm.CMD_SYNC_MSG_ACK, MsgSvrSyncMsgAckHandler, ctx)
}

/******************************************************************************
 **函数名称: Launch
 **功    能: 启动OLSVR服务
 **输入参数: NONE
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func (ctx *MsgSvrCntx) Launch() {
	ctx.proxy.Launch()
}

/******************************************************************************
 **函数名称: log_init
 **功    能: 初始化日志
 **输入参数: NONE
 **输出参数: NONE
 **返    回:
 **     err: 日志对象
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:34:34 #
 ******************************************************************************/
func (ctx *MsgSvrCntx) log_init() (err error) {
	conf := ctx.conf

	ctx.log = logs.NewLogger(20000)
	log := ctx.log

	err = os.Mkdir("../log", 0755)
	if nil != err && false == os.IsExist(err) {
		log.Emergency(err.Error())
		return err
	}

	log.SetLogger("file", fmt.Sprintf(`{"filename":"%s/../log/olsvr.log"}`, conf.AppPath))
	log.SetLevel(logs.LevelDebug)
	return nil
}
