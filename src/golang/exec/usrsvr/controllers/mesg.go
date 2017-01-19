package controllers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/protobuf/proto"

	"beehive-im/src/golang/lib/chat"
	"beehive-im/src/golang/lib/comm"
	"beehive-im/src/golang/lib/crypt"
	"beehive-im/src/golang/lib/im"
	"beehive-im/src/golang/lib/mesg"
)

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
// 通用请求

////////////////////////////////////////////////////////////////////////////////
// 上线请求

type OnlineToken struct {
	uid uint64 /* 用户ID */
	ttl int64  /* TTL */
	sid uint64 /* 会话SID */
}

/******************************************************************************
 **函数名称: online_token_decode
 **功    能: 解码TOKEN
 **输入参数:
 **     token: TOKEN字串
 **输出参数: NONE
 **返    回: TOKEN字段
 **实现描述: 解析token, 并提取有效数据.
 **注意事项:
 **     TOKEN的格式"uid:${uid}:ttl:${ttl}:sid:${sid}:end"
 **     uid: 用户ID
 **     ttl: 该token的最大生命时间
 **     sid: 会话SID
 **作    者: # Qifeng.zou # 2016.11.20 09:28:06 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) online_token_decode(token string) *OnlineToken {
	tk := &OnlineToken{}

	/* > TOKEN解码 */
	cry := crypt.CreateEncodeCtx(ctx.conf.Cipher)
	orig_token := crypt.Decode(cry, token)
	words := strings.Split(orig_token, ":")
	if 7 != len(words) {
		ctx.log.Error("Token format not right! token:%s orig:%s", token, orig_token)
		return nil
	}

	ctx.log.Debug("token:%s orig:%s", token, orig_token)

	/* > 验证TOKEN合法性 */
	uid, _ := strconv.ParseInt(words[1], 10, 64)
	tk.uid = uint64(uid)
	ctx.log.Debug("words[1]:%s uid:%d", words[1], tk.uid)

	ttl, _ := strconv.ParseInt(words[3], 10, 64)
	tk.ttl = int64(ttl)
	ctx.log.Debug("words[3]:%s ttl:%d", words[3], tk.ttl)

	sid, _ := strconv.ParseInt(words[5], 10, 64)
	tk.sid = uint64(sid)
	ctx.log.Debug("words[5]:%s sid:%d sid:%d", words[5], sid, tk.sid)

	return tk
}

/******************************************************************************
 **函数名称: online_req_check
 **功    能: 检验ONLINE请求合法性
 **输入参数:
 **     req: ONLINE请求
 **输出参数: NONE
 **返    回: 异常信息
 **实现描述: 计算TOKEN合法性
 **注意事项:
 **     1.TOKEN的格式"${uid}:${ttl}:${sid}"
 **         uid: 用户ID
 **         ttl: 该token的最大生命时间
 **         sid: 会话SID
 **     2.头部数据(MesgHeader)中的SID此时表示的是客户端的连接CID.
 **作    者: # Qifeng.zou # 2016.11.02 10:20:57 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) online_req_check(req *mesg.MesgOnlineReq) error {
	token := ctx.online_token_decode(req.GetToken())
	if nil == token {
		ctx.log.Error("Decode token failed!")
		return errors.New("Decode token failed!")
	} else if token.ttl < time.Now().Unix() {
		ctx.log.Error("Token is timeout! uid:%d sid:%d ttl:%d", token.uid, token.sid, token.ttl)
		return errors.New("Token is timeout!")
	} else if uint64(token.uid) != req.GetUid() || uint64(token.sid) != req.GetSid() {
		ctx.log.Error("Token is invalid! uid:%d/%d sid:%d/%d ttl:%d",
			token.uid, req.GetUid(), token.sid, req.GetSid(), token.ttl)
		return errors.New("Token is invalid!!")
	}

	return nil
}

/******************************************************************************
 **函数名称: online_parse
 **功    能: 解析上线请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **     code: 错误码
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) online_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgOnlineReq, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of online is invalid!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	ctx.log.Debug("Online request header! cmd:0x%04X flag:%d length:%d chksum:0x%08X cid:%d nid:%d serial:%d head:%d",
		head.GetCmd(), head.GetFlag(), head.GetLength(),
		head.GetChkSum(), head.GetSid(), head.GetNid(),
		head.GetSerial(), comm.MESG_HEAD_SIZE)

	/* > 解析PB协议 */
	req = &mesg.MesgOnlineReq{}
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal body failed! errmsg:%s", err.Error())
		return head, nil, comm.ERR_SVR_BODY_INVALID, errors.New("Unmarshal body failed!")
	}

	/* > 校验协议合法性 */
	err = ctx.online_req_check(req)
	if nil != err {
		ctx.log.Error("Check online-request failed!")
		return head, nil, comm.ERR_SVR_CHECK_FAIL, err
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: send_err_online_ack
 **功    能: 发送上线应答
 **输入参数:
 **     head: 协议头
 **     req: 上线请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 ** {
 **     required uint64 uid = 1;        // M|用户ID|数字|
 **     required uint64 sid = 2;        // M|连接ID|数字|内部使用
 **     required string app = 3;        // M|APP名|字串|
 **     required string version = 4;    // M|APP版本|字串|
 **     optional uint32 terminal = 5;   // O|终端类型|数字|(0:未知 1:PC 2:TV 3:手机)|
 **     optional uint32 code = 6;     // M|错误码|数字|
 **     optional string errmsg = 7;     // M|错误描述|字串|
 ** }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_online_ack(head *comm.MesgHeader,
	req *mesg.MesgOnlineReq, code uint32, errmsg string) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	rsp := &mesg.MesgOnlineAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	if nil != req {
		rsp.Uid = proto.Uint64(req.GetUid())
		rsp.Sid = proto.Uint64(req.GetSid())
		rsp.App = proto.String(req.GetApp())
		rsp.Version = proto.String(req.GetVersion())
		rsp.Terminal = proto.Uint32(req.GetTerminal())
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ONLINE_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ONLINE_ACK, p.Buff, uint32(len(p.Buff)))

	ctx.log.Debug("Send online ack succ!")

	return 0
}

/******************************************************************************
 **函数名称: send_online_ack
 **功    能: 发送上线应答
 **输入参数:
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 ** {
 **     required uint64 uid = 1;        // M|用户ID|数字|
 **     required uint64 sid = 2;        // M|会话ID|数字|内部使用
 **     required string app = 3;        // M|APP名|字串|
 **     required string version = 4;    // M|APP版本|字串|
 **     optional uint32 terminal = 5;   // O|终端类型|数字|(0:未知 1:PC 2:TV 3:手机)|
 **     optional uint32 code = 6;     // M|错误码|数字|
 **     optional string errmsg = 7;     // M|错误描述|字串|
 ** }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_online_ack(head *comm.MesgHeader, req *mesg.MesgOnlineReq) int {
	/* > 设置协议体 */
	rsp := &mesg.MesgOnlineAck{
		Uid:      proto.Uint64(req.GetUid()),
		Sid:      proto.Uint64(req.GetSid()),
		App:      proto.String(req.GetApp()),
		Version:  proto.String(req.GetVersion()),
		Terminal: proto.Uint32(req.GetTerminal()),
		Code:     proto.Uint32(0),
		Errmsg:   proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ONLINE_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ONLINE_ACK, p.Buff, uint32(len(p.Buff)))

	ctx.log.Debug("Send online ack success!")

	return 0
}

/******************************************************************************
 **函数名称: online_handler
 **功    能: 上线处理
 **输入参数:
 **     req: 上线请求
 **输出参数: NONE
 **返    回: 异常信息
 **实现描述:
 **     1. 校验是否SID上线信息是否存在冲突. 如果存在冲突, 则将之前的连接踢下线.
 **     2. 更新数据库信息
 **注意事项:
 **     1. 在上线请求中, head中的sid此时为侦听层cid
 **     2. 在上线请求中, req中的sid此时为会话sid
 **作    者: # Qifeng.zou # 2016.11.01 21:12:36 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) online_handler(head *comm.MesgHeader, req *mesg.MesgOnlineReq) (err error) {
	var key string

	rds := ctx.redis.Get()
	defer rds.Close()

	pl := ctx.redis.Get()
	defer func() {
		pl.Do("")
		pl.Close()
	}()

	ttl := time.Now().Unix() + comm.CHAT_SID_TTL

	/* 获取会话属性 */
	attr, err := im.GetSidAttr(ctx.redis, req.GetSid())
	if nil != err {
		ctx.send_kick(head.GetCid(), head.GetNid(), comm.ERR_SYS_SYSTEM, err.Error())
		return
	} else if (0 != attr.Uid && attr.Uid != req.GetUid()) ||
		(0 != attr.Nid && attr.Nid != head.GetNid()) { // 注意：当nid为0时表示会话SID之前并未登录.
		ctx.log.Error("Session's nid is conflict! uid:%d sid:%d nid:[%d/%d] cid:%d",
			attr.Uid, req.GetSid(), attr.Nid, head.GetNid(), head.GetCid())
		/* 清理会话数据 */
		im.CleanSidData(ctx.redis, head.GetSid())
		/* 将老连接踢下线 */
		ctx.send_kick(req.GetSid(), attr.Nid, comm.ERR_SVR_DATA_COLLISION, "Session's nid is collision!")
	}

	/* 记录SID集合 */
	pl.Send("ZADD", comm.IM_KEY_SID_ZSET, ttl, req.GetSid())

	/* 记录UID集合 */
	pl.Send("ZADD", comm.IM_KEY_UID_ZSET, ttl, req.GetUid())

	/* 记录SID->UID/NID */
	key = fmt.Sprintf(comm.IM_KEY_SID_ATTR, req.GetSid())
	pl.Send("HMSET", key, "UID", req.GetUid(), "NID", head.GetNid())

	/* 记录UID->SID集合 */
	key = fmt.Sprintf(comm.IM_KEY_UID_TO_SID_SET, req.GetUid())
	pl.Send("SADD", key, req.GetSid())

	return err
}

/******************************************************************************
 **函数名称: UsrSvrOnlineReqHandler
 **功    能: 上线请求
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **请求协议:
 **     {
 **        required uint64 uid = 1;         // M|用户ID|数字|
 **        required uint64 sid = 2;         // M|会话ID|数字|
 **        required string token = 3;       // M|鉴权TOKEN|字串|
 **        required string app = 4;         // M|APP名|字串|
 **        required string version = 5;     // M|APP版本|字串|
 **        optional uint32 terminal = 6;    // O|终端类型|数字|(0:未知 1:PC 2:TV 3:手机)|
 **     }
 **注意事项:
 **     1. 首先需要调用MesgHeadNtoh()对头部数据进行直接序转换.
 **     2. 在上线请求中, head中的sid此时为侦听层cid
 **     3. 在上线请求中, req中的sid此时为会话sid
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func UsrSvrOnlineReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv online request! cmd:0x%04X dest:%d length:%d", cmd, dest, length)

	/* > 解析上线请求 */
	head, req, code, err := ctx.online_parse(data)
	if nil != err {
		ctx.log.Error("Parse online request failed! errmsg:%s", err.Error())
		ctx.send_err_online_ack(head, req, code, err.Error())
		return -1
	}

	/* > 初始化上线环境 */
	err = ctx.online_handler(head, req)
	if nil != err {
		ctx.log.Error("Online handler failed!")
		ctx.send_err_online_ack(head, req, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}

	/* > 发送上线应答 */
	ctx.send_online_ack(head, req)

	return 0
}

////////////////////////////////////////////////////////////////////////////////
// 下线请求

/******************************************************************************
 **函数名称: offline_parse
 **功    能: 解析Offline请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.02 22:17:38 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) offline_parse(data []byte) (head *comm.MesgHeader) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		ctx.log.Error("Header of offline-req is invalid!")
		return nil
	}

	return head
}

/******************************************************************************
 **函数名称: offline_handler
 **功    能: Offline处理
 **输入参数:
 **     head: 协议头
 **输出参数: NONE
 **返    回: 异常信息
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.11 23:23:50 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) offline_handler(head *comm.MesgHeader) error {
	return im.CleanSidData(ctx.redis, head.GetSid())
}

/******************************************************************************
 **函数名称: UsrSvrOfflineReqHandler
 **功    能: 下线请求
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func UsrSvrOfflineReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv offline request!")

	/* 1. > 解析下线请求 */
	head := ctx.offline_parse(data)
	if nil == head {
		ctx.log.Error("Parse offline request failed!")
		return -1
	}

	/* 2. > 清理会话数据 */
	err := ctx.offline_handler(head)
	if nil != err {
		ctx.log.Error("Offline handler failed!")
		return -1
	}

	return 0
}

////////////////////////////////////////////////////////////////////////////////
// PING请求

/******************************************************************************
 **函数名称: ping_parse
 **功    能: 解析PING请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回: 协议头
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 21:18:29 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) ping_parse(data []byte) (head *comm.MesgHeader) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		ctx.log.Error("Header of ping is invalid!")
		return nil
	}

	return head
}

/******************************************************************************
 **函数名称: ping_handler
 **功    能: PING处理
 **输入参数:
 **     head: 协议头
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 更新会话相关的TTL. 如果发现数据异常, 则需要清除该会话的数据, 并将该
 **          会话踢下线.
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 21:53:38 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) ping_handler(head *comm.MesgHeader) {
	code, err := im.UpdateSidData(ctx.redis, head.GetNid(), head.GetSid())
	if nil != err {
		im.CleanSidData(ctx.redis, head.GetSid()) // 清理会话数据
		ctx.send_kick(head.GetSid(), head.GetNid(), code, err.Error())
	}
}

/******************************************************************************
 **函数名称: UsrSvrPingHandler
 **功    能: 客户端PING
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项: 由侦听层给各终端回复PONG请求
 **作    者: # Qifeng.zou # 2016.11.03 21:40:30 #
 ******************************************************************************/
func UsrSvrPingHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv ping request!")

	/* > 解析PING请求 */
	head := ctx.ping_parse(data)
	if nil == head {
		ctx.log.Error("Parse ping request failed!")
		return -1
	}

	/* > PING请求处理 */
	ctx.ping_handler(head)

	return 0
}

/******************************************************************************
 **函数名称: send_kick
 **功    能: 发送踢人操作
 **输入参数:
 **     sid: 会话ID
 **     nid: 结点ID
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 ** {
 **     required int code = 1;          // M|错误码|数字|
 **     required string errmsg = 2;     // M|错误描述|数字|
 ** }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.12.16 20:49:02 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_kick(sid uint64, nid uint32, code uint32, errmsg string) int {
	var head comm.MesgHeader

	ctx.log.Debug("Send kick command! sid:%d nid:%d", sid, nid)

	/* > 设置协议体 */
	rsp := &mesg.MesgKickReq{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_KICK_REQ
	head.Sid = sid
	head.Nid = nid
	head.Length = uint32(length)

	comm.MesgHeadHton(&head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_KICK_REQ, p.Buff, uint32(len(p.Buff)))

	return 0
}

/* 订阅请求 */
func UsrSvrSubReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 取消订阅请求 */
func UsrSvrUnsubReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
// 申请序列号

/******************************************************************************
 **函数名称: alloc_seq_parse
 **功    能: 解析ALLOC-SEQ请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **     code: 错误码
 **     errmsg: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 11:27:25 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) alloc_seq_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgAllocSeq, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of alloc-seq failed!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	ctx.log.Debug("Alloc-seq request header! cmd:0x%04X flag:%d length:%d chksum:0x%08X cid:%d nid:%d serial:%d head:%d",
		head.GetCmd(), head.GetFlag(), head.GetLength(),
		head.GetChkSum(), head.GetSid(), head.GetNid(),
		head.GetSerial(), comm.MESG_HEAD_SIZE)

	/* > 解析PB协议 */
	req = &mesg.MesgAllocSeq{}
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal alloc-seq request failed! errmsg:%s", err.Error())
		return head, nil, comm.ERR_SVR_BODY_INVALID, errors.New("Unmarshal alloc-seq request failed!")
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: alloc_seq_handler
 **功    能: ALLOC-SEQ处理
 **输入参数:
 **     head: 协议头
 **     req: 请求数据
 **输出参数: NONE
 **返    回: 异常信息
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.11 23:23:50 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) alloc_seq_handler(
	head *comm.MesgHeader, req *mesg.MesgAllocSeq) (seq uint64, err error) {
	rds := ctx.redis.Get()
	defer rds.Close()

	/* > 申请用户消息序列号 */
	key := fmt.Sprintf(comm.CHAT_KEY_USR_MSGID_INCR, req.GetUid())

	seq_str, err := redis.String(rds.Do("INCRBY", key, req.GetNum()))
	if nil != err {
		ctx.log.Error("Alloc seq failed! errmsg:%s", err.Error())
		return 0, err
	}

	seq_int, err := strconv.ParseInt(seq_str, 10, 64)
	if nil != err {
		ctx.log.Error("Parse seq-str failed! errmsg:%s", err.Error())
		return 0, err
	} else if 0 == seq {
		ctx.log.Error("Alloced seq is invalid!")
		return 0, errors.New("Alloced seq is invalid!")
	}

	seq = uint64(seq_int) - uint64(req.GetNum())

	return seq, nil
}

/******************************************************************************
 **函数名称: send_err_alloc_seq_ack
 **功    能: 发送ALLOC-SEQ错误应答
 **输入参数:
 **     head: 协议头
 **     req: ALLOC-SEQ请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;        // M|用户ID|数字|<br>
 **         required uint64 seq = 2;        // M|序列号起始值|数字|<br>
 **         required uint16 num = 3;        // M|分配序列号个数|数字|<br>
 **         required uint32 code = 4;       // M|错误码|数字|<br>
 **         required string errmsg = 5;     // M|错误描述|字串|<br>
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 11:34:17 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_alloc_seq_ack(head *comm.MesgHeader,
	req *mesg.MesgAllocSeq, code uint32, errmsg string) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	rsp := &mesg.MesgAllocSeqAck{
		Seq:    proto.Uint64(0),
		Num:    proto.Uint32(0),
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	if nil != req {
		rsp.Uid = proto.Uint64(req.GetUid())
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ALLOC_SEQ_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ALLOC_SEQ_ACK, p.Buff, uint32(len(p.Buff)))

	ctx.log.Debug("Send alloc-seq ack success!")

	return 0
}

/******************************************************************************
 **函数名称: send_alloc_seq_ack
 **功    能: 发送ALLOC-SEQ应答
 **输入参数:
 **     head: 头部数据
 **     req: 请求数据
 **     seq: 申请到的序列号
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **     {
 **         required uint64 uid = 1;        // M|用户ID|数字|<br>
 **         required uint64 seq = 2;        // M|序列号起始值|数字|<br>
 **         required uint16 num = 3;        // M|分配序列号个数|数字|<br>
 **         required uint32 code = 4;       // M|错误码|数字|<br>
 **         required string errmsg = 5;     // M|错误描述|字串|<br>
 **     }

 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_alloc_seq_ack(head *comm.MesgHeader, req *mesg.MesgAllocSeq, seq uint64) int {
	/* > 设置协议体 */
	rsp := &mesg.MesgAllocSeqAck{
		Uid:    proto.Uint64(req.GetUid()),
		Seq:    proto.Uint64(seq),
		Num:    proto.Uint32(req.GetNum()),
		Code:   proto.Uint32(0),
		Errmsg: proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ALLOC_SEQ_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ALLOC_SEQ_ACK, p.Buff, uint32(len(p.Buff)))

	ctx.log.Debug("Send alloc-seq ack success!")

	return 0
}

/******************************************************************************
 **函数名称: UsrSvrAllocSeqHandler
 **功    能: 申请序列号请求
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 08:48:51 #
 ******************************************************************************/
func UsrSvrAllocSeqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv alloc-seq request!")

	/* > 解析ALLOC-SEQ请求 */
	head, req, code, err := ctx.alloc_seq_parse(data)
	if nil != err {
		ctx.log.Error("Parse alloc-seq request failed! errmsg:%s", err.Error())
		ctx.send_err_alloc_seq_ack(head, req, code, err.Error())
		return -1
	} else if 0 == req.GetNum() {
		ctx.log.Error("Alloc seq num is zero! uid:%d nid:%d", req.GetUid(), head.GetNid())
		ctx.send_err_alloc_seq_ack(head, req, comm.ERR_SVR_PARSE_PARAM, "Alloc seq num is zero!")
		return -1
	}

	/* > 校验合法性 */
	attr, err := im.GetSidAttr(ctx.redis, head.GetSid())
	if nil != err {
		ctx.log.Error("Get sid attribute failed! errmsg:%s", err.Error())
		ctx.send_err_alloc_seq_ack(head, req, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	} else if req.GetUid() != attr.Uid || head.GetNid() != attr.Nid {
		ctx.log.Error("Data is collision! uid:%d/%d nid:%d/%d",
			attr.Uid, req.GetUid(), attr.Nid, head.GetNid())
		ctx.send_err_alloc_seq_ack(head, req, comm.ERR_SYS_SYSTEM, "Data is collision!")
		return -1
	}

	/* > 申请序列号 */
	seq, err := ctx.alloc_seq_handler(head, req)
	if nil != err {
		ctx.log.Error("Alloc seq handler failed!")
		ctx.send_err_alloc_seq_ack(head, req, comm.ERR_SYS_SYSTEM, err.Error())
		return -1
	}

	/* > 发送申请序列号应答 */
	ctx.send_alloc_seq_ack(head, req, seq)

	return 0
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
// 私聊处理

////////////////////////////////////////////////////////////////////////////////
/* 加入黑名单 */

/******************************************************************************
 **函数名称: blacklist_add_parse
 **功    能: 解析BLACKLIST-ADD请求
 **输入参数:
 **     data: 原始数据
 **输出参数: NONE
 **返    回:
 **     head: 协议头
 **     req: 请求内容
 **     code: 错误码
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.19 10:06:14 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) blacklist_add_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgBlacklistAdd, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of blacklist-add is invalid!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	/* > 解析PB协议 */
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal body of blacklist-add failed! errmsg:%s", err.Error())
		return head, nil, comm.ERR_SVR_BODY_INVALID, err
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: blacklist_add_handler
 **功    能: 进行BLACKLIST-ADD处理
 **输入参数:
 **     head: 协议头
 **     req: 请求内容
 **输出参数: NONE
 **返    回:
 **     code: 错误码
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.19 10:30:04 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) blacklist_add_handler(
	head *comm.MesgHeader, req *mesg.MesgBlacklistAdd) (code uint32, err error) {
	rds := ctx.redis.Get()
	defer rds.Close()

	/* > 加入用户黑名单 */
	key := fmt.Sprintf(comm.CHAT_KEY_USR_BLACKLIST_ZSET, req.GetOrig())

	_, err = rds.Do("ZADD", key, req.GetDest())
	if nil != err {
		ctx.log.Error("Add into blacklist failed! errmsg:%s", err.Error())
		return comm.ERR_SYS_SYSTEM, err
	}

	return 0, nil
}

/******************************************************************************
 **函数名称: send_err_blacklist_add_ack
 **功    能: 发送BLACKLIST-ADD应答
 **输入参数:
 **     head: 协议头
 **     req: BLACKLIST-ADD请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 ** {
 **     required uint32 code = 1;       // M|错误码|数字|
 **     required string errmsg = 2;     // M|错误描述|字串|
 ** }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_blacklist_add_ack(head *comm.MesgHeader,
	req *mesg.MesgBlacklistAdd, code uint32, errmsg string) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	ack := &mesg.MesgOnlineAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(ack)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_BLACKLIST_ADD_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_BLACKLIST_ADD_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: send_blacklist_add_ack
 **功    能: 发送BLACKLIST-ADD应答
 **输入参数:
 **     head: 协议头
 **     req: BLACKLIST-ADD请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 ** {
 **     required uint32 code = 1;       // M|错误码|数字|
 **     required string errmsg = 2;     // M|错误描述|字串|
 ** }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.19 10:40:03 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_blacklist_add_ack(
	head *comm.MesgHeader, req *mesg.MesgBlacklistAdd) int {
	/* > 设置协议体 */
	ack := &mesg.MesgOnlineAck{
		Code:   proto.Uint32(0),
		Errmsg: proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(ack)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_BLACKLIST_ADD_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_BLACKLIST_ADD_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: UsrSvrBlacklistAddHandler
 **功    能: 加入黑名单
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: 0:成功 !0:失败
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.19 09:49:10 #
 ******************************************************************************/
func UsrSvrBlacklistAddHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	/* > 解析BLACKLIST-ADD请求 */
	head, req, code, err := ctx.blacklist_add_parse(data)
	if nil != err {
		ctx.log.Error("Parse blacklist-add failed! code:%d errmsg:%s", code, err.Error())
		ctx.send_err_blacklist_add_ack(head, req, code, err.Error())
		return -1
	}

	/* > 验证请求合法性 */
	attr, err := im.GetSidAttr(ctx.redis, head.GetSid())
	if nil != err {
		ctx.log.Error("Get attr by sid failed! errmsg:%s", err.Error())
		ctx.send_err_blacklist_add_ack(head, req, code, err.Error())
		return -1
	} else if 0 != attr.Uid && attr.Uid != req.GetOrig() {
		errmsg := "Uid is collision!"
		ctx.log.Error("errmsg:%s", errmsg)
		ctx.send_err_blacklist_add_ack(head, req, comm.ERR_SYS_SYSTEM, errmsg)
		return -1
	}

	/* > 进行BLACKLIST-ADD处理 */
	code, err = ctx.blacklist_add_handler(head, req)
	if nil != err {
		ctx.log.Error("Handle blacklist-add failed! code:%d errmsg:%s", code, err.Error())
		ctx.send_err_blacklist_add_ack(head, req, code, err.Error())
		return -1
	}

	/* > 发送BLACKLIST-ADD应答 */
	ctx.send_blacklist_add_ack(head, req)

	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 移除黑名单 */
func UsrSvrBlacklistDelHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 设置禁言 */
func UsrSvrBanAddHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 解除禁言 */
func UsrSvrBanDelHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
// 群聊处理

////////////////////////////////////////////////////////////////////////////////
/* 创建群组 */
func UsrSvrGroupCreatHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 解散群组 */
func UsrSvrGroupDismissHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 申请入群 */
func UsrSvrGroupApplyHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 退群 */
func UsrSvrGroupQuitHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 邀请入群 */
func UsrSvrGroupInviteHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 群组踢人 */
func UsrSvrGroupKickHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 群组禁言 */
func UsrSvrGroupBanAddHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 解除群组禁言 */
func UsrSvrGroupBanDelHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 加入群组黑名单 */
func UsrSvrGroupBlacklistAddHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 移除群组黑名单 */
func UsrSvrGroupBlacklistDelHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 添加群组管理员 */
func UsrSvrGroupMgrAddHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 移除群组管理员 */
func UsrSvrGroupMgrDelHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 群组成员列表 */
func UsrSvrGroupUsrListHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
// 聊天室

////////////////////////////////////////////////////////////////////////////////
/* 创建聊天室 */
func UsrSvrRoomCreatHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

/* 解散聊天室 */
func UsrSvrRoomDismissHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 加入聊天室 */

/******************************************************************************
 **函数名称: room_join_parse
 **功    能: 解析ROOM-JOIN请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **     code: 错误码
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 16:41:17 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_join_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgRoomJoin, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of room-join failed!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	/* > 解析PB协议 */
	req = &mesg.MesgRoomJoin{}
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal join request failed! errmsg:%s", err.Error())
		return head, nil, comm.ERR_SVR_HEAD_INVALID, err
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: send_err_room_join_ack
 **功    能: 发送ROOM-JOIN应答(异常)
 **输入参数:
 **     head: 协议头
 **     req: ROOM-JOIN请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **         required uint64 rid = 2;    // M|聊天室ID|数字|
 **         required uint32 gid = 3;    // M|分组ID|数字|
 **         optional uint32 code = 4; // M|错误码|数字|
 **         optional string errmsg = 5; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 17:12:36 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_room_join_ack(head *comm.MesgHeader,
	req *mesg.MesgRoomJoin, code uint32, err error) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	rsp := &mesg.MesgRoomJoinAck{
		Gid:    proto.Uint32(0),
		Code:   proto.Uint32(code),
		Errmsg: proto.String(err.Error()),
	}

	if nil != req {
		rsp.Uid = proto.Uint64(req.GetUid())
		rsp.Rid = proto.Uint64(req.GetRid())
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_JOIN_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_JOIN_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: send_room_join_ack
 **功    能: 发送上线应答
 **输入参数:
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **         required uint64 rid = 2;    // M|聊天室ID|数字|
 **         required uint32 gid = 3;    // M|分组ID|数字|
 **         optional uint32 code = 4; // M|错误码|数字|
 **         optional string errmsg = 5; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_room_join_ack(head *comm.MesgHeader, req *mesg.MesgRoomJoin, gid uint32) int {
	/* > 设置协议体 */
	rsp := &mesg.MesgRoomJoinAck{
		Uid:    proto.Uint64(req.GetUid()),
		Rid:    proto.Uint64(req.GetRid()),
		Gid:    proto.Uint32(gid),
		Code:   proto.Uint32(0),
		Errmsg: proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_JOIN_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_JOIN_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: alloc_gid
 **功    能: 分配组ID
 **输入参数:
 **     rid: 聊天室ID
 **输出参数: NONE
 **返    回: 组ID
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 20:08:06 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) alloc_gid(rid uint64) (gid uint32, err error) {
	var num int

	rds := ctx.redis.Get()
	defer rds.Close()

	key := fmt.Sprintf(comm.CHAT_KEY_RID_GID_TO_NUM_ZSET, rid)

	/* > 优先加入到gid为0的分组 */
	num, err = redis.Int(rds.Do("ZSCORE", key, "0"))
	if uint32(num) < comm.CHAT_ROOM_GROUP_MAX_NUM {
		return 0, nil
	}

	/* > 获取有序GID列表: 以人数从多到少进行排序(加入人数最少的分组) */
	min := 0
	max := comm.CHAT_ROOM_GROUP_MAX_NUM - 1

	gid_lst, err := redis.Ints(rds.Do("ZRANGEBYSCORE", key, min, max, "LIMIT", 0, 1))
	if nil != err {
		ctx.log.Error("Get group list failed! errmsg:%s", err)
		return 0, err
	} else if len(gid_lst) > 0 {
		return uint32(gid_lst[0]), nil
	}

	grp_num, err := redis.Int(rds.Do("ZCARD", key))
	if nil != err {
		ctx.log.Error("Get group num failed! errmsg:%s", err)
		return 0, err
	}

	return uint32(grp_num), nil
}

/******************************************************************************
 **函数名称: room_join_handler
 **功    能: ROOM-JOIN处理
 **输入参数:
 **     head: 协议头
 **     req: ROOM-JOIN请求
 **输出参数: NONE
 **返    回: 组ID
 **实现描述:
 **注意事项: 已验证了ROOM-JOIN请求的合法性
 **作    者: # Qifeng.zou # 2016.11.03 19:51:46 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_join_handler(
	head *comm.MesgHeader, req *mesg.MesgRoomJoin) (gid uint32, err error) {
	rds := ctx.redis.Get()
	defer rds.Close()

	pl := ctx.redis.Get()
	defer func() {
		pl.Do("")
		pl.Close()
	}()

	/* > 判断UID是否在黑名单中 */
	key := fmt.Sprintf(comm.CHAT_KEY_ROOM_USR_BLACKLIST_SET, req.GetRid())
	ok, err := redis.Bool(rds.Do("SISMEMBER", key, req.GetUid()))
	if nil != err {
		ctx.log.Error("Exec command [SISMEMBER] failed! rid:%d uid:%d err:",
			req.GetRid(), req.GetUid(), err.Error())
		return 0, err
	} else if true == ok {
		ctx.log.Error("User is in blacklist! rid:%d uid:%d", req.GetRid(), req.GetUid())
		return 0, errors.New("User is in blacklist!")
	}

GET_GID:
	/* > 判断UID是否登录过 */
	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_RID, req.GetUid())
	ok, err = redis.Bool(rds.Do("HEXISTS", key, req.GetRid()))
	if nil != err {
		ctx.log.Error("Get rid [%d] by uid failed!", req.GetRid())
		return 0, err
	} else if true == ok {
		gid_int, err := redis.Int(rds.Do("HGET", key, req.GetRid()))
		if nil != err {
			ctx.log.Error("Get rid [%d] by uid failed!", req.GetRid())
			return 0, err
		}

		key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_UID_SID_ZSET, req.GetRid())
		member := fmt.Sprintf(comm.UID_SID_STR, req.GetUid(), head.GetSid())
		ttl := time.Now().Unix() + comm.CHAT_SID_TTL
		pl.Send("ZADD", key, ttl, member) // 加入RID -> UID集合"${uid}:${sid}"

		key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_SID_ZSET, req.GetRid())
		pl.Send("ZADD", key, ttl, head.GetSid()) // 加入RID -> SID集合

		key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_NID_ZSET, req.GetRid())
		pl.Send("ZADD", key, ttl, head.GetNid()) // 加入RID -> NID集合

		return uint32(gid_int), nil
	}

	/* > 分配新的分组 */
	gid, err = ctx.alloc_gid(req.GetRid())
	if nil != err {
		ctx.log.Error("Alloc gid failed! rid:%d", req.GetRid())
		return 0, err
	}

	/* > 设置UID的RID组GID */
	key = fmt.Sprintf(comm.CHAT_KEY_UID_TO_RID, req.GetUid())
	ok, err = redis.Bool(rds.Do("HSETNX", key, req.GetRid(), gid)) /* 防止冲突 */
	if nil != err {
		ctx.log.Error("Get rid [%d] by uid failed!", req.GetRid())
		return 0, err
	} else if false == ok {
		goto GET_GID /* 存在冲突 */
	}

	/* > 更新数据库统计 */
	key = fmt.Sprintf(comm.CHAT_KEY_RID_GID_TO_NUM_ZSET, req.GetRid())
	pl.Send("ZINCRBY", key, 1, gid)

	key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_UID_SID_ZSET, req.GetRid())
	member := fmt.Sprintf(comm.UID_SID_STR, req.GetUid(), head.GetSid())
	ttl := time.Now().Unix() + comm.CHAT_SID_TTL
	pl.Send("ZADD", key, ttl, member) // 加入RID -> UID集合"${uid}:${sid}"

	key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_SID_ZSET, req.GetRid())
	pl.Send("ZADD", key, ttl, head.GetSid()) // 加入RID -> SID集合

	key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_NID_ZSET, req.GetRid())
	pl.Send("ZADD", key, ttl, head.GetNid()) // 加入RID -> NID集合

	return gid, nil
}

/******************************************************************************
 **函数名称: UsrSvrRoomJoinReqHandler
 **功    能: 加入聊天室
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **请求协议:
 **     {
 **        required uint64 uid = 1;    // M|用户ID|数字|
 **        required uint64 rid = 2;    // M|聊天室ID|数字|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func UsrSvrRoomJoinReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv join request! cmd:0x%04X dest:%d length:%d", cmd, dest, length)

	/* 1. > 解析ROOM-JOIN请求 */
	head, req, code, err := ctx.room_join_parse(data)
	if nil == req {
		ctx.log.Error("Parse room-join request failed!")
		ctx.send_err_room_join_ack(head, req, code, err)
		return -1
	}

	/* 2. > 初始化上线环境 */
	gid, err := ctx.room_join_handler(head, req)
	if nil != err {
		ctx.log.Error("Room join handler failed!")
		ctx.send_err_room_join_ack(head, req, comm.ERR_SYS_SYSTEM, err)
		return -1
	}

	/* 3. > 发送上线应答 */
	ctx.send_room_join_ack(head, req, gid)

	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 退出聊天室 */

/******************************************************************************
 **函数名称: room_quit_isvalid
 **功    能: 判断ROOM-QUIT是否合法
 **输入参数:
 **     req: ROOM-QUIT请求
 **输出参数: NONE
 **返    回: true:合法 false:非法
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 21:26:22 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_quit_isvalid(req *mesg.MesgRoomQuit) bool {
	if 0 == req.GetUid() || 0 == req.GetRid() {
		return false
	}
	return true
}

/******************************************************************************
 **函数名称: send_err_room_quit_ack
 **功    能: 发送ROOM-QUIT应答(异常)
 **输入参数:
 **     head: 协议头
 **     req: ROOM-QUIT请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **        required uint64 uid = 1;    // M|用户ID|数字|
 **        required uint64 rid = 2;    // M|聊天室ID|数字|
 **        required uint32 gid = 3;    // M|分组ID|数字|
 **        optional uint32 code = 4; // M|错误码|数字|
 **        optional string errmsg = 5; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 21:20:34 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_room_quit_ack(head *comm.MesgHeader,
	req *mesg.MesgRoomQuit, code uint32, errmsg string) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	rsp := &mesg.MesgRoomQuitAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	if nil != req {
		rsp.Uid = proto.Uint64(req.GetUid())
		rsp.Rid = proto.Uint64(req.GetRid())
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_QUIT_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_QUIT_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: room_quit_parse
 **功    能: 解析ROOM-QUIT请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 21:18:29 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_quit_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgRoomQuit, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of room-quit failed!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	/* > 解析PB协议 */
	req = &mesg.MesgRoomQuit{}
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal room quit request failed! errmsg:%s", err.Error())
		return nil, nil, comm.ERR_SVR_BODY_INVALID, errors.New("Parse body failed!")
	}

	/* > 校验协议合法性 */
	if !ctx.room_quit_isvalid(req) {
		ctx.log.Error("Room quit request is invalid!")
		return nil, nil, comm.ERR_SVR_CHECK_FAIL, errors.New("Check request failed!")
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: send_room_quit_ack
 **功    能: 发送ROOM-QUIT应答
 **输入参数:
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **         required uint64 rid = 2;    // M|聊天室ID|数字|
 **         required uint32 gid = 3;    // M|分组ID|数字|
 **         optional uint32 code = 4; // M|错误码|数字|
 **         optional string errmsg = 5; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.01 18:37:59 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_room_quit_ack(head *comm.MesgHeader, req *mesg.MesgRoomQuit) int {
	/* > 设置协议体 */
	rsp := &mesg.MesgRoomQuitAck{
		Uid:    proto.Uint64(req.GetUid()),
		Rid:    proto.Uint64(req.GetRid()),
		Code:   proto.Uint32(0),
		Errmsg: proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_QUIT_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_QUIT_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: room_quit_handler
 **功    能: 退出聊天室处理
 **输入参数:
 **     head: 协议头
 **     req: ROOM-QUIT请求
 **输出参数: NONE
 **返    回:
 **     code: 错误码
 **     err: 错误描述
 **实现描述:
 **注意事项: 已验证了ROOM-QUIT请求的合法性
 **作    者: # Qifeng.zou # 2016.11.03 21:28:18 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_quit_handler(
	head *comm.MesgHeader, req *mesg.MesgRoomQuit) (code uint32, err error) {
	pl := ctx.redis.Get()
	defer func() {
		pl.Do("")
		pl.Close()
	}()

	key := fmt.Sprintf(comm.CHAT_KEY_RID_TO_SID_ZSET, req.GetRid())
	pl.Send("ZREM", key, head.GetSid()) // 清理RID -> SID集合

	key = fmt.Sprintf(comm.CHAT_KEY_RID_TO_UID_SID_ZSET, req.GetRid())
	member := fmt.Sprintf(comm.UID_SID_STR, req.GetUid(), head.GetSid())
	pl.Send("ZREM", key, member) // 清理RID -> UID集合"${uid}:${sid}"

	return 0, nil
}

/******************************************************************************
 **函数名称: UsrSvrRoomQuitReqHandler
 **功    能: 退出聊天室
 **输入参数:
 **     cmd: 消息类型
 **     orig: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **请求协议:
 **     {
 **        required uint64 uid = 1;    // M|用户ID|数字|
 **        required uint64 rid = 2;    // M|聊天室ID|数字|
 **     }
 **注意事项: 需要对协议头进行字节序转换
 **作    者: # Qifeng.zou # 2016.10.30 22:32:23 #
 ******************************************************************************/
func UsrSvrRoomQuitReqHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv room quit request!")

	/* 1. > 解析ROOM-QUIT请求 */
	head, req, code, err := ctx.room_quit_parse(data)
	if nil != err {
		ctx.log.Error("Parse room quit request failed!")
		ctx.send_err_room_quit_ack(head, req, code, err.Error())
		return -1
	}

	/* 2. > 退出聊天室处理 */
	code, err = ctx.room_quit_handler(head, req)
	if nil != err {
		ctx.log.Error("Hanle room quit request failed!")
		ctx.send_err_room_quit_ack(head, req, code, err.Error())
		return -1
	}

	/* 3. > 发送ROOM-QUIT应答 */
	ctx.send_room_quit_ack(head, req)

	return 0
}

////////////////////////////////////////////////////////////////////////////////
/* 踢出聊天室 */

/******************************************************************************
 **函数名称: room_kick_parse
 **功    能: 解析KICK请求
 **输入参数:
 **     data: 接收的数据
 **输出参数: NONE
 **返    回:
 **     head: 通用协议头
 **     req: 协议体内容
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 23:21:37 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_kick_parse(data []byte) (
	head *comm.MesgHeader, req *mesg.MesgRoomKick, code uint32, err error) {
	/* > 字节序转换 */
	head = comm.MesgHeadNtoh(data)
	if !comm.MesgHeadIsValid(head) {
		errmsg := "Header of room-kick is invalid!"
		ctx.log.Error(errmsg)
		return nil, nil, comm.ERR_SVR_HEAD_INVALID, errors.New(errmsg)
	}

	/* > 解析PB协议 */
	req = &mesg.MesgRoomKick{}
	err = proto.Unmarshal(data[comm.MESG_HEAD_SIZE:], req)
	if nil != err {
		ctx.log.Error("Unmarshal room-kick request failed! errmsg:%s", err.Error())
		return head, nil, comm.ERR_SVR_BODY_INVALID, err
	}

	return head, req, 0, nil
}

/******************************************************************************
 **函数名称: send_err_room_kick_ack
 **功    能: 发送KICK应答(异常)
 **输入参数:
 **     head: 协议头
 **     req: ROOM-KICK请求
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **         required uint64 rid = 2;    // M|聊天室ID|数字|
 **         required uint32 code = 3;   // M|错误码|数字|
 **         required string errmsg = 4; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.03 17:12:36 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_err_room_kick_ack(head *comm.MesgHeader,
	req *mesg.MesgRoomKick, code uint32, errmsg string) int {
	if nil == head {
		return -1
	}

	/* > 设置协议体 */
	rsp := &mesg.MesgRoomKickAck{
		Code:   proto.Uint32(code),
		Errmsg: proto.String(errmsg),
	}

	if nil != req {
		rsp.Uid = proto.Uint64(req.GetUid())
		rsp.Rid = proto.Uint64(req.GetRid())
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_KICK_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_KICK_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: send_room_kick_ack
 **功    能: 发送ROOM-KICK应答
 **输入参数:
 **     head: 协议头
 **     req: 请求数据
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **应答协议:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **         required uint64 rid = 2;    // M|聊天室ID|数字|
 **         required uint32 code = 3;   // M|错误码|数字|
 **         required string errmsg = 4; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 23:32:20 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_room_kick_ack(head *comm.MesgHeader, req *mesg.MesgRoomKick) int {
	/* > 设置协议体 */
	rsp := &mesg.MesgRoomKickAck{
		Uid:    proto.Uint64(req.GetUid()),
		Rid:    proto.Uint64(req.GetRid()),
		Code:   proto.Uint32(0),
		Errmsg: proto.String("Ok"),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	head.Cmd = comm.CMD_ROOM_KICK_ACK
	head.Length = uint32(length)

	comm.MesgHeadHton(head, p)
	copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

	/* > 发送协议包 */
	ctx.frwder.AsyncSend(comm.CMD_ROOM_KICK_ACK, p.Buff, uint32(len(p.Buff)))

	return 0
}

/******************************************************************************
 **函数名称: send_room_kick_notice
 **功    能: 发送ROOM-KICK-NTC通知
 **输入参数:
 **     head: 协议头
 **     req: 请求数据
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **协议格式:
 **     {
 **         required uint64 uid = 1;    // M|用户ID|数字|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.13 07:46:08 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) send_room_kick_notice(head *comm.MesgHeader, req *mesg.MesgRoomKick) int {
	rds := ctx.redis.Get()
	defer rds.Close()

	/* > 设置协议体 */
	rsp := &mesg.MesgRoomKickNtc{
		Uid: proto.Uint64(req.GetUid()),
	}

	/* 生成PB数据 */
	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return -1
	}

	length := len(body)

	/* > 拼接协议包 */
	p := &comm.MesgPacket{}
	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	nid_list, err := redis.Ints(rds.Do("ZRANGEBYSCORE", comm.IM_KEY_LSN_NID_ZSET, "-inf", "+inf"))
	if nil != err {
		ctx.log.Error("Get listen nid failed! errmsg:%s", err.Error())
		return -1
	}

	num := len(nid_list)

	for idx := 0; idx < num; idx += 1 {
		head.Cmd = comm.CMD_ROOM_KICK_NTC
		head.Length = uint32(length)
		head.Sid = req.GetRid() // 会话ID改为聊天室ID
		head.Nid = uint32(nid_list[idx])
		head.ChkSum = comm.MSG_CHKSUM_VAL

		comm.MesgHeadHton(head, p)
		copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

		/* > 发送协议包 */
		ctx.frwder.AsyncSend(comm.CMD_ROOM_KICK_NTC, p.Buff, uint32(len(p.Buff)))
	}

	return 0
}

/******************************************************************************
 **函数名称: room_kick_handler
 **功    能: ROOM-KICK处理
 **输入参数:
 **     head: 协议头
 **     req: ROOM-KICK请求
 **输出参数: NONE
 **返    回:
 **     code: 错误码
 **     err: 错误信息
 **实现描述:
 **注意事项: 已验证了ROOM-KICK请求的合法性
 **作    者: # Qifeng.zou # 2017.01.12 23:34:28 #
 ******************************************************************************/
func (ctx *UsrSvrCntx) room_kick_handler(
	head *comm.MesgHeader, req *mesg.MesgRoomKick) (code uint32, err error) {
	rds := ctx.redis.Get()
	defer rds.Close()

	pl := ctx.redis.Get()
	defer func() {
		pl.Do("")
		pl.Close()
	}()

	/* > 获取会话属性 */
	attr, err := im.GetSidAttr(ctx.redis, head.GetSid())
	if nil != err {
		ctx.log.Error("Get sid attr failed! rid:%d uid:%d errmsg:%s",
			req.GetRid(), req.GetUid(), err.Error())
		return comm.ERR_SYS_SYSTEM, err
	} else if !chat.IsRoomManager(ctx.redis, req.GetRid(), attr.Uid) {
		ctx.log.Error("You're not owner! rid:%d uid:%d", req.GetRid(), req.GetUid())
		return comm.ERR_SYS_PERM_DENIED, errors.New("You're not room owner!")
	}

	/* > 用户加入黑名单 */
	key := fmt.Sprintf(comm.CHAT_KEY_ROOM_USR_BLACKLIST_SET, req.GetRid())

	pl.Send("ZADD", key, req.GetUid())

	return 0, nil
}

/******************************************************************************
 **函数名称: UsrSvrRoomKickHandler
 **功    能: 踢出聊天室
 **输入参数:
 **     cmd: 消息类型
 **     dest: 业务层ID
 **     data: 收到数据
 **     length: 数据长度
 **     param: 附加参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述:
 **请求协议:
 **     {
 **        required uint64 uid = 1;    // M|用户ID|数字|
 **        required uint64 rid = 2;    // M|聊天室ID|数字|
 **        required uint32 code = 3;   // M|错误码|数字|
 **        required string errmsg = 4; // M|错误描述|字串|
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.12 23:58:49 #
 ******************************************************************************/
func UsrSvrRoomKickHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	ctx, ok := param.(*UsrSvrCntx)
	if false == ok {
		return -1
	}

	ctx.log.Debug("Recv room-kick request! cmd:0x%04X dest:%d length:%d", cmd, dest, length)

	/* > 解析ROOM-KICK请求 */
	head, req, code, err := ctx.room_kick_parse(data)
	if nil != err {
		ctx.log.Error("Parse room-kick request failed!")
		ctx.send_err_room_kick_ack(head, req, code, err.Error())
		return -1
	}

	/* > 执行ROOM-KICK操作 */
	code, err = ctx.room_kick_handler(head, req)
	if nil != err {
		ctx.log.Error("Room kick handler failed!")
		ctx.send_err_room_kick_ack(head, req, code, err.Error())
		return -1
	}

	/* > 发送ROOM-KICK应答 */
	ctx.send_room_kick_ack(head, req)

	/* > 发送ROOM-KICK-NTC通知 */
	ctx.send_room_kick_notice(head, req)

	return 0
}

/* 获取聊天室人数 */
func UsrSvrRoomUsrNumHandler(cmd uint32, dest uint32, data []byte, length uint32, param interface{}) int {
	return 0
}
