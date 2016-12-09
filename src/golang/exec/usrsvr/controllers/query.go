package controllers

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/astaxie/beego"

	"beehive-im/src/golang/lib/comm"
	"beehive-im/src/golang/lib/crypt"
)

/* 获取IP列表 */
type UsrSvrQueryCtrl struct {
	BaseController
}

func (this *UsrSvrQueryCtrl) Query() {
	ctx := GetUsrSvrCtx()

	opt := this.GetString("opt")
	switch opt {
	case "iplist":
		this.query_iplist(ctx)
		return
	}

	this.unsupport_option(opt)
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

type UsrSvrUnsupportRsp struct {
	Code   int    `json:"code"`   // 错误码
	ErrMsg string `json:"errmsg"` // 错误描述
}

/* 未知选项应答 */
func (this *UsrSvrQueryCtrl) unsupport_option(opt string) {
	var resp UsrSvrUnsupportRsp

	resp.Code = comm.ERR_SVR_INVALID_PARAM
	resp.ErrMsg = fmt.Sprintf("Unsupport option [%s].", opt)

	this.Data["json"] = &resp
	this.ServeJSON()
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

/* 注册参数 */
type UsrSvrIpListParam struct {
	uid      uint64 // 用户ID
	sid      uint64 // 会话ID
	clientip string // 客户端IP
}

func (this *UsrSvrQueryCtrl) query_iplist(ctx *UsrSvrCntx) {
	/* > 提取注册参数 */
	param, err := this.iplist_parse_param(ctx)
	if nil != err {
		ctx.log.Error("Parse param failed! uid:%d sid:%d clientip:%s",
			param.uid, param.sid, param.clientip)
		this.iplist_fail(param, comm.ERR_SVR_PARSE_PARAM, err.Error())
		return
	}

	ctx.log.Debug("Param list. uid:%d sid:%d clientip:%s", param.uid, param.sid, param.clientip)

	/* > 获取IP列表 */
	this.iplist_handler(ctx, param)

	return
}

/******************************************************************************
 **函数名称: iplist_parse_param
 **功    能: 解析参数
 **输入参数:
 **     ctx: 上下文
 **输出参数: NONE
 **返    回:
 **     param: 注册参数
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.25 23:17:52 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_parse_param(ctx *UsrSvrCntx) (*UsrSvrIpListParam, error) {
	var param UsrSvrIpListParam

	/* > 提取注册参数 */
	id, _ := this.GetInt64("uid")
	param.uid = uint64(id)

	id, _ = this.GetInt64("sid")
	param.sid = uint64(id)

	param.clientip = this.GetString("clientip")

	/* > 校验参数合法性 */
	if 0 == param.uid || 0 == param.sid || "" == param.clientip {
		ctx.log.Error("Paramter is invalid. uid:%d sid:%d clientip:%s",
			param.uid, param.sid, param.clientip)
		return &param, errors.New("Paramter is invalid!")
	}

	ctx.log.Debug("Get ip list param. uid:%d sid:%d clientip:%s",
		param.uid, param.sid, param.clientip)

	return &param, nil
}

/* IP列表应答 */
type UsrSvrIpListRsp struct {
	Uid    uint64   `json:"uid"`    // 用户ID
	Sid    uint64   `json:"sid"`    // 会话ID
	Token  string   `json:"token"`  // 鉴权TOKEN
	Expire int      `json:"expire"` // 过期时长
	Len    int      `json:"len"`    // 列表长度
	List   []string `json:"list"`   // IP列表
	Code   int      `json:"code"`   // 错误码
	ErrMsg string   `json:"errmsg"` // 错误描述
}

/******************************************************************************
 **函数名称: iplist_handler
 **功    能: 获取IP列表
 **输入参数:
 **     ctx: 上下文
 **     param: URL参数
 **输出参数:
 **返    回: NONE
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.24 17:00:07 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_handler(ctx *UsrSvrCntx, param *UsrSvrIpListParam) {
	iplist := this.iplist_get(ctx, param.clientip)
	if nil == iplist {
		ctx.log.Error("Get ip list failed!")
		this.iplist_fail(param, comm.ERR_SYS_SYSTEM, "Get ip list failed!")
		return
	}

	this.iplist_success(param, iplist)

	return
}

/******************************************************************************
 **函数名称: iplist_fail
 **功    能: 应答错误信息
 **输入参数:
 **     param: 注册参数
 **     code: 错误码
 **     errmsg: 错误描述
 **输出参数:
 **返    回: NONE
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.25 23:13:09 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_fail(param *UsrSvrIpListParam, code int, errmsg string) {
	var resp UsrSvrIpListRsp

	resp.Uid = param.uid
	resp.Sid = 0
	resp.Code = code
	resp.ErrMsg = errmsg

	this.Data["json"] = &resp
	this.ServeJSON()
}

/******************************************************************************
 **函数名称: iplist_success
 **功    能: 应答处理成功
 **输入参数:
 **     param: 注册参数
 **     iplist: IP列表
 **输出参数:
 **返    回: NONE
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.11.25 23:13:02 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_success(param *UsrSvrIpListParam, iplist []string) {
	var resp UsrSvrIpListRsp

	resp.Uid = param.uid
	resp.Sid = param.sid
	resp.Token = this.iplist_token(param)
	resp.Expire = comm.TIME_DAY
	resp.Len = len(iplist)
	resp.List = iplist
	resp.Code = 0
	resp.ErrMsg = "OK"

	this.Data["json"] = &resp
	this.ServeJSON()
}

/******************************************************************************
 **函数名称: iplist_token
 **功    能: 生成TOKEN字串
 **输入参数:
 **输出参数: NONE
 **返    回:
 **     token: 加密TOKEN
 **     expire: 过期时间
 **实现描述:
 **注意事项:
 **     TOKEN的格式"uid:${uid}:ttl:${ttl}:sid:${sid}:end"
 **     uid: 用户ID
 **     ttl: 该token的最大生命时间
 **     sid: 会话UID
 **作    者: # Qifeng.zou # 2016.11.25 23:54:27 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_token(param *UsrSvrIpListParam) string {
	ctx := GetUsrSvrCtx()
	ttl := time.Now().Unix() + comm.TIME_DAY

	/* > 原始TOKEN */
	token := fmt.Sprintf("uid:%d:ttl:%d:sid:%d:end", param.uid, ttl, param.sid)

	/* > 加密TOKEN */
	cry := crypt.CreateEncodeCtx(ctx.conf.Cipher)

	return crypt.Encode(cry, token)
}

/******************************************************************************
 **函数名称: iplist_get
 **功    能: 获取IP列表
 **输入参数:
 **     ctx: 上下文
 **     clientip: 客户端IP
 **输出参数: NONE
 **返    回: IP列表
 **实现描述:
 **注意事项: 加读锁
 **作    者: # Qifeng.zou # 2016.11.27 07:42:54 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_get(ctx *UsrSvrCntx, clientip string) []string {
	item := ctx.ipdict.Query(clientip)
	if nil == item {
		ctx.lsnlist.RLock()
		defer ctx.lsnlist.RUnlock()
		return this.iplist_get_def(ctx)
	}

	ctx.lsnlist.RLock()
	defer ctx.lsnlist.RUnlock()

	/* > 获取国家/地区下辖的运营商列表 */
	operators, ok := ctx.lsnlist.list[item.GetNation()]
	if nil == operators || !ok {
		return this.iplist_get_def(ctx)
	}

	/* > 获取运营商下辖的侦听层列表 */
	lsn_list, ok := operators[item.GetOperator()]
	if nil == lsn_list || !ok {
		return this.iplist_get_def(ctx)
	}

	items := make([]string, 0)
	items = append(items, lsn_list[rand.Intn(len(lsn_list))])
	return items
}

/******************************************************************************
 **函数名称: iplist_get_def
 **功    能: 获取默认IP列表
 **输入参数:
 **     ctx: 上下文
 **     clientip: 客户端IP
 **输出参数: NONE
 **返    回: IP列表
 **实现描述:
 **注意事项: 外部已经加读锁
 **作    者: # Qifeng.zou # 2016.11.27 19:33:49 #
 ******************************************************************************/
func (this *UsrSvrQueryCtrl) iplist_get_def(ctx *UsrSvrCntx) []string {
	var ok bool

	/* > 获取"默认"国家/地区下辖的运营商列表 */
	operators, ok := ctx.lsnlist.list["CN"]
	if nil == operators || 0 == len(operators) || !ok {
		ctx.log.Error("Get default iplist by nation failed!")
		return nil
	}

	for k, v := range operators {
		ctx.log.Debug("k:%s v:%s", k, v)
	}

	/* > 获取"默认"运营商下辖的侦听层列表 */
	lsn_list, ok := operators["中国电信"]
	if nil == lsn_list || 0 == len(lsn_list) || !ok {
		ctx.log.Error("Get default iplist by operator failed!")
		return nil
	}

	items := make([]string, 0)
	items = append(items, lsn_list[rand.Intn(len(lsn_list))])
	return items
}

////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////
