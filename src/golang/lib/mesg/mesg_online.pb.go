// Code generated by protoc-gen-go.
// source: mesg_online.proto
// DO NOT EDIT!

/*
Package mesg_online is a generated protocol buffer package.

It is generated from these files:
	mesg_online.proto

It has these top-level messages:
	MesgOnlineReq
*/
package mesg_online

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type MesgOnlineReq struct {
	Uid              *uint64 `protobuf:"varint,1,opt,name=uid" json:"uid,omitempty"`
	Token            *string `protobuf:"bytes,2,opt,name=token" json:"token,omitempty"`
	App              *string `protobuf:"bytes,3,opt,name=app" json:"app,omitempty"`
	Version          *string `protobuf:"bytes,4,opt,name=version" json:"version,omitempty"`
	Terminal         *uint32 `protobuf:"varint,5,opt,name=terminal" json:"terminal,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *MesgOnlineReq) Reset()         { *m = MesgOnlineReq{} }
func (m *MesgOnlineReq) String() string { return proto.CompactTextString(m) }
func (*MesgOnlineReq) ProtoMessage()    {}

func (m *MesgOnlineReq) GetUid() uint64 {
	if m != nil && m.Uid != nil {
		return *m.Uid
	}
	return 0
}

func (m *MesgOnlineReq) GetToken() string {
	if m != nil && m.Token != nil {
		return *m.Token
	}
	return ""
}

func (m *MesgOnlineReq) GetApp() string {
	if m != nil && m.App != nil {
		return *m.App
	}
	return ""
}

func (m *MesgOnlineReq) GetVersion() string {
	if m != nil && m.Version != nil {
		return *m.Version
	}
	return ""
}

func (m *MesgOnlineReq) GetTerminal() uint32 {
	if m != nil && m.Terminal != nil {
		return *m.Terminal
	}
	return 0
}
