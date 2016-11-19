package comm

//#聊天室系统REDIS键值定义列表
const (
	//|**宏**|**键值**|**类型**|**描述**|**备注**|
	CHAT_KEY_SID_ZSET       = "chat:sid:zset"          //*| ZSET | 会话SID集合 | 成员:SID/分值:TTL |
	CHAT_KEY_UID_ZSET       = "chat:uid:zset"          //| ZSET | 用户UID集合 | 成员:UID/分值:TTL |
	CHAT_KEY_SID_INCR       = "chat:sid:incr"          //*| STRING | 会话SID增量器 | 只增不减 注意:sid不能为0 |
	CHAT_KEY_SID_ATTR       = "chat:sid:%d:attr"       //*| HASH | 会话SID属性 | 包含UID/NID |
	CHAT_KEY_UID_TO_SID_SET = "chat:uid:%d:to:sid:set" //| SET | 用户UID对应的会话SID集合 | SID集合 |
	//|**宏**|**键值**|**类型**|**描述**|**备注**|
	//聊天室
	CHAT_KEY_RID_ZSET             = "chat:rid:zset"               //*| ZSET | 聊天室RID集合 | 成员:RID/分值:TTL |
	CHAT_KEY_UID_TO_RID           = "chat:uid:%d:to:rid:htab"     //| HASH | 用户UID对应的RID集合 | 成员:RID/分值:GID |
	CHAT_KEY_SID_TO_RID_ZSET      = "chat:sid:%d:to:rid:zset"     //*| ZSET | 会话SID对应的RID集合 | 成员:RID/分值:GID |
	CHAT_KEY_ROOM_GROUP_USR_NUM   = "chat:room:group:usr:num"     //| ZSET | 聊天室分组人数配置 | 成员:RID/分值:USERNUM |
	CHAT_KEY_RID_GID_TO_NUM_ZSET  = "chat:rid:%d:to:gid:num:zset" //*| ZSET | 某聊天室各组人数 | 成员:GID/分值:USERNUM |
	CHAT_KEY_RID_TO_NID_ZSET      = "chat:rid:%d:to:nid:zset"     //*| ZSET | 某聊天室->帧听层 | 成员:NID/分值:TTL |
	CHAT_KEY_RID_NID_TO_NUM_ZSET  = "chat:rid:%d:nid:to:num:zset" //*| ZSET | 某聊天室各帧听层人数 | 成员:NID/分值:USERNUM | 由帧听层上报数据获取
	CHAT_KEY_RID_SUB_USR_NUM_ZSET = "chat:rid:sub:usr:num:zset"   //| ZSET | 聊天室人数订阅集合 | 暂无 |
	CHAT_KEY_RID_TO_UID_ZSET      = "chat:rid:%d:to:uid:zset"     //| ZSET | 聊天室用户列表 | 成员:UID/分值:TTL |
	CHAT_KEY_RID_TO_SID_ZSET      = "chat:rid:%d:to:sid:zset"     //| ZSET | 聊天室SID列表 | 成员:SID/分值:TTL |
	//|**宏**|**键值**|**类型**|**描述**|**备注**|
	//群聊
	CHAT_KEY_GID_ZSET            = "chat:gid:zset"               //| ZSET | 群ID集合 | 成员:GID/分值:TTL |
	CHAT_KEY_UID_TO_GID          = "chat:uid:%d:to:gid:htab"     //| HASH | 用户UID对应的GID集合 | 成员:GID/分值:0 |
	CHAT_KEY_GID_TO_NID_ZSET     = "chat:gid:%d:to:nid:zset"     //| ZSET | 某群->帧听层 | 成员:NID/分值:TTL |
	CHAT_KEY_GID_NID_TO_NUM_ZSET = "chat:gid:%d:nid:to:num:zset" //| ZSET | 某群各帧听层人数 | 成员:NID/分值:USERNUM | 由帧听层上报数据获取
	CHAT_KEY_GID_TO_UID_ZSET     = "chat:gid:%d:to:uid:zset"     //| ZSET | 某群在线用户列表 | 成员:UID/分值:TTL |
	CHAT_KEY_GID_TO_SID_ZSET     = "chat:gid:%d:to:sid:zset"     //| ZSET | 某群SID列表 | 成员:SID/分值:TTL |
	//|**宏**|**键值**|**类型**|**描述**|**备注**|
	CHAT_KEY_LSN_OP_ZSET        = "chat:lsn:op:zset"           //| ZSET | 帧听层运营商集合 | 成员:运营商ID/分值:TTL |
	CHAT_KEY_LSN_OP_TO_NID_ZSET = "chat:lsn:op:%d:to:nid:zset" //| ZSET | 运营商帧听层NID集合 | 成员:NID/分值:TTL |
	CHAT_KEY_LSN_NID_ZSET       = "chat:lsn:nid:zset"          //| ZSET | 帧听层NID集合 | 成员:NID/分值:TTL |
	CHAT_KEY_LSN_NID_TO_ADDR    = "chat:lsn:nid:to:addr"       //| HASH | 帧听层NID->地址 | 键:NID/值:外网IP+端口 |
	CHAT_KEY_LSN_ADDR_TO_NID    = "chat:lsn:addr:to:nid"       //| HASH | 帧听层地址->NID | 键:外网IP+端口/值:NID |
	CHAT_KEY_FRWD_NID_ZSET      = "chat:frwd:nid:zset"         //| ZSET | 转发层NID集合 | 成员:NID/分值:TTL |
	CHAT_KEY_FRWD_NID_TO_ADDR   = "chat:frwd:nid:to:addr"      //| HASH | 转发层NID->地址 | 键:NID/值:内网IP+端口 |
	CHAT_KEY_FRWD_ADDR_TO_NID   = "chat:frwd:addr:to:nid"      //| HASH | 转发层地址->NID | 键:内网IP+端口/值:NID |
	CHAT_KEY_PREC_RNUM_ZSET     = "chat:prec:rnum:zset"        //| ZSET | 人数统计精度 | 成员:prec/分值:记录条数 |
	CHAT_KEY_PREC_USR_MAX_NUM   = "chat:prec:%d:usr:max:num"   //| HASH | 某统计精度最大人数 | 键:时间/值:最大人数 |
	CHAT_KEY_PREC_USR_MIN_NUM   = "chat:prec:%d:usr:min:num"   //| HASH | 某统计精度最少人数 | 键:时间/值:最少人数 |
)
