package controllers

import (
	"errors"
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/golang/protobuf/proto"

	"beehive-im/src/golang/lib/comm"
	"beehive-im/src/golang/lib/mesg"
)

/* 推送接口 */
type UsrSvrPushCtrl struct {
	BaseController
}

func (this *UsrSvrPushCtrl) Push() {
	ctx := GetUsrSvrCtx()

	dim := this.GetString("dim")
	switch dim {
	case "sid": // SID推送
		this.SidPush(ctx)
		return
	case "uid": // UID推送
		this.UidPush(ctx)
		return
	case "appid": // APP推送
		this.AppPush(ctx)
		return
	case "room": // ROOM推送
		this.RoomPush(ctx)
		return
	case "group": // GROUP推送
		this.GroupPush(ctx)
		return
	case "broadcast": // 全员推送
		this.Broadcast(ctx)
		return
	}

	errmsg := fmt.Sprintf("Unsupport this dimension:%s.", dim)

	this.Error(comm.ERR_SVR_INVALID_PARAM, errmsg)
}

////////////////////////////////////////////////////////////////////////////////
// SID推送

func (this *UsrSvrPushCtrl) SidPush(ctx *UsrSvrCntx) {
	return
}

////////////////////////////////////////////////////////////////////////////////
// UID推送

/* UID推送请求 */
type UidPushReq struct {
	ctrl *UsrSvrPushCtrl
}

/* UID推送参数 */
type UidPushParam struct {
	uid    uint64 // 用户UID
	expire uint32 // 超时时间
}

/* UID推送应答 */
type UidPushRsp struct {
	Uid    uint64 `json:"uid"`    // 用户UID
	Code   int    `json:"code"`   // 错误码
	ErrMsg string `json:"errmsg"` // 错误描述
}

func (this *UsrSvrPushCtrl) UidPush(ctx *UsrSvrCntx) {
	req := &UidPushReq{ctrl: this}

	/* > 提取广播参数 */
	param, err := req.parse_param(ctx)
	if nil != err {
		ctx.log.Error("Parse uid push param failed! uid:%d", param.uid)
		this.Error(comm.ERR_SVR_PARSE_PARAM, err.Error())
		return
	}

	/* > UID推送处理 */
	code, err := req.push_handler(ctx, param)
	if nil != err {
		this.Error(code, err.Error())
		return
	}

	req.push_success(param)

	return

}

/******************************************************************************
 **函数名称: parse_param
 **功    能: 解析参数
 **输入参数:
 **     ctx: 上下文
 **输出参数: NONE
 **返    回:
 **     param: 注册参数
 **     err: 错误描述
 **实现描述: 从url参数中抽取
 **注意事项:
 **作    者: # Qifeng.zou # 2017.03.19 22:05:58 #
 ******************************************************************************/
func (req *UidPushReq) parse_param(
	ctx *UsrSvrCntx) (param *UidPushParam, err error) {
	this := req.ctrl
	param = &UidPushParam{}

	/* > 提取注册参数 */
	uid, _ := this.GetInt64("uid")
	param.uid = uint64(uid)

	expire, _ := this.GetInt32("expire")
	param.expire = uint32(expire)

	/* > 校验参数合法性 */
	if 0 == param.uid {
		ctx.log.Error("Paramter is invalid. uid:%d", param.uid)
		return param, errors.New("Paramter is invalid!")
	}

	return param, nil
}

/******************************************************************************
 **函数名称: push_handler
 **功    能: UID推送处理
 **输入参数:
 **     param: 请求参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 按照协议返回http应答
 **注意事项:
 **作    者: # Qifeng.zou # 2017.03.19 22:19:04 #
 ******************************************************************************/
func (req *UidPushReq) push_handler(ctx *UsrSvrCntx, param *UidPushParam) (code int, err error) {
	return 0, nil
}

/******************************************************************************
 **函数名称: push_success
 **功    能: UID推送成功
 **输入参数:
 **     param: 请求参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 按照协议返回http应答
 **注意事项:
 **作    者: # Qifeng.zou # 2017.03.19 22:19:04 #
 ******************************************************************************/
func (req *UidPushReq) push_success(param *UidPushParam) {
	this := req.ctrl
	rsp := &UidPushRsp{
		Uid:    param.uid,
		Code:   0,
		ErrMsg: "OK",
	}

	this.Data["json"] = rsp
	this.ServeJSON()
}

////////////////////////////////////////////////////////////////////////////////
// APP推送

func (this *UsrSvrPushCtrl) AppPush(ctx *UsrSvrCntx) {
	return
}

////////////////////////////////////////////////////////////////////////////////
// ROOM推送

/* ROOM推送请求 */
type RoomPushReq struct {
	ctrl *UsrSvrPushCtrl
}

/* ROOM推送参数 */
type RoomPushParam struct {
	rid    uint64 // 聊天室ID
	expire uint32 // 超时时间
}

/* ROOM推送应答 */
type RoomPushRsp struct {
	Rid    uint64 `json:"rid"`    // 聊天室ID
	Code   int    `json:"code"`   // 错误码
	ErrMsg string `json:"errmsg"` // 错误描述
}

func (this *UsrSvrPushCtrl) RoomPush(ctx *UsrSvrCntx) {
	req := &RoomPushReq{ctrl: this}

	/* > 提取参数 */
	param, err := req.prase_param(ctx)
	if nil != err {
		ctx.log.Error("Parse room push param failed! rid:%d", param.rid)
		this.Error(comm.ERR_SVR_PARSE_PARAM, err.Error())
		return
	}

	/* > ROOM推送处理 */
	code, err := req.push_handler(ctx, param)
	if nil != err {
		this.Error(code, err.Error())
		return
	}

	req.push_success(param)

	return
}

/******************************************************************************
 **函数名称: prase_param
 **功    能: 解析参数
 **输入参数:
 **     ctx: 上下文
 **输出参数: NONE
 **返    回:
 **     param: 注册参数
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.14 11:10:59 #
 ******************************************************************************/
func (req *RoomPushReq) prase_param(
	ctx *UsrSvrCntx) (param *RoomPushParam, err error) {
	this := req.ctrl
	param = &RoomPushParam{}

	/* > 提取注册参数 */
	rid, _ := this.GetInt64("rid")
	param.rid = uint64(rid)

	expire, _ := this.GetInt32("expire")
	param.expire = uint32(expire)

	/* > 校验参数合法性 */
	if 0 == param.rid {
		ctx.log.Error("Paramter is invalid. rid:%d", param.rid)
		return param, errors.New("Paramter is invalid!")
	}

	return param, nil
}

/******************************************************************************
 **函数名称: push_handler
 **功    能: ROOM推送请求的处理
 **输入参数:
 **     ctx: 上下文
 **     param: URL参数
 **输出参数:
 **返    回:
 **     code: 错误码
 **     err: 错误信息
 **实现描述:
 **     {
 **         required uint64 rid = 1;        // M|聊天室ID
 **         required uint32 level = 2;      // M|消息级别
 **         required uint64 time = 3;       // M|发送时间
 **         required uint32 expire = 4;     // M|过期时间
 **         required bytes data = 5;        // M|透传数据
 **     }
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.14 12:27:53 #
 ******************************************************************************/
func (req *RoomPushReq) push_handler(
	ctx *UsrSvrCntx, param *RoomPushParam) (code int, err error) {
	this := req.ctrl

	rds := ctx.redis.Get()
	defer rds.Close()

	pl := ctx.redis.Get()
	defer func() {
		pl.Do("")
		pl.Close()
	}()

	ctm := time.Now().Unix()
	ttl := ctm + int64(param.expire)
	data := this.Ctx.Input.RequestBody

	/* > 申请聊天室消息序列号  */
	key := fmt.Sprintf(comm.CHAT_KEY_ROOM_MSGID_INCR, param.rid)

	msgid, err := redis.Uint64(rds.Do("INCRBY", key, 1))
	if nil != err {
		ctx.log.Error("Get room msgid failed! key:%s errmsg:%s", key, err.Error())
		return comm.ERR_SYS_SYSTEM, err
	}

	/* > 生成PB数据 */
	rsp := &mesg.MesgRoomBc{
		Rid:    proto.Uint64(param.rid),    // 聊天室ID
		Msgid:  proto.Uint64(msgid),        // 消息ID
		Level:  proto.Uint32(0),            // 优先级别
		Time:   proto.Uint64(uint64(ctm)),  // 发送时间
		Expire: proto.Uint32(param.expire), // 超时时间
		Data:   []byte(data),               // 透传内容
	}

	body, err := proto.Marshal(rsp)
	if nil != err {
		ctx.log.Error("Marshal protobuf failed! errmsg:%s", err.Error())
		return comm.ERR_SYS_SYSTEM, err
	}

	/* > 放入聊天室广播集合 */
	key = fmt.Sprintf(comm.CHAT_KEY_ROOM_BC_HASH, param.rid)
	pl.Send("HSETNX", key, msgid, body)

	key = fmt.Sprintf(comm.CHAT_KEY_ROOM_BC_ZSET, param.rid)
	pl.Send("ZADD", key, ttl, msgid)

	/* > 申请内存空间 */
	length := len(body)
	p := &comm.MesgPacket{}

	p.Buff = make([]byte, comm.MESG_HEAD_SIZE+length)

	/* > 获取侦听层集合 */
	nid_list, err := redis.Ints(rds.Do("ZRANGEBYSCORE", comm.IM_KEY_LSND_NID_ZSET, ctm, "+inf"))
	if nil != err {
		ctx.log.Error("Get listen nid failed! errmsg:%s", err.Error())
		return comm.ERR_SYS_SYSTEM, err
	}

	num := len(nid_list)

	for idx := 0; idx < num; idx += 1 {
		head := &comm.MesgHeader{}

		head.Cmd = comm.CMD_ROOM_BC
		head.Sid = param.rid // 会话ID改为聊天室ID
		head.Length = uint32(length)
		head.Nid = uint32(nid_list[idx])

		comm.MesgHeadHton(head, p)
		copy(p.Buff[comm.MESG_HEAD_SIZE:], body)

		/* > 发送协议包 */
		ctx.frwder.AsyncSend(comm.CMD_ROOM_BC, p.Buff, uint32(len(p.Buff)))
	}

	return 0, nil
}

/******************************************************************************
 **函数名称: push_success
 **功    能: 聊天室推送成功
 **输入参数:
 **     param: 请求参数
 **输出参数: NONE
 **返    回: VOID
 **实现描述: 按照协议返回http应答
 **注意事项:
 **作    者: # Qifeng.zou # 2017.01.14 16:00:42 #
 ******************************************************************************/
func (req *RoomPushReq) push_success(param *RoomPushParam) {
	this := req.ctrl
	rsp := &RoomPushRsp{
		Rid:    param.rid,
		Code:   0,
		ErrMsg: "OK",
	}

	this.Data["json"] = rsp
	this.ServeJSON()
}

////////////////////////////////////////////////////////////////////////////////
// GROUP推送

/* GROUP推送参数 */
type GroupPushParam struct {
	gid    uint64 // 群组ID
	expire uint32 // 超时时间
}

/* GROUP推送应答 */
type GroupPushRsp struct {
	Gid    uint64 `json:"gid"`    // 群组ID
	Code   int    `json:"code"`   // 错误码
	ErrMsg string `json:"errmsg"` // 错误描述
}

func (this *UsrSvrPushCtrl) GroupPush(ctx *UsrSvrCntx) {
	return
}

////////////////////////////////////////////////////////////////////////////////
// 全员推送

func (this *UsrSvrPushCtrl) Broadcast(ctx *UsrSvrCntx) {
	return
}
