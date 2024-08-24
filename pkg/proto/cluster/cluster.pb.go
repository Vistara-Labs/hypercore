// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.3
// source: pkg/proto/cluster.proto

package cluster

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ClusterEvent int32

const (
	ClusterEvent_ERROR ClusterEvent = 0
	ClusterEvent_SPAWN ClusterEvent = 1
)

// Enum value maps for ClusterEvent.
var (
	ClusterEvent_name = map[int32]string{
		0: "ERROR",
		1: "SPAWN",
	}
	ClusterEvent_value = map[string]int32{
		"ERROR": 0,
		"SPAWN": 1,
	}
)

func (x ClusterEvent) Enum() *ClusterEvent {
	p := new(ClusterEvent)
	*p = x
	return p
}

func (x ClusterEvent) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ClusterEvent) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_cluster_proto_enumTypes[0].Descriptor()
}

func (ClusterEvent) Type() protoreflect.EnumType {
	return &file_pkg_proto_cluster_proto_enumTypes[0]
}

func (x ClusterEvent) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ClusterEvent.Descriptor instead.
func (ClusterEvent) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{0}
}

type ClusterMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Event          ClusterEvent `protobuf:"varint,1,opt,name=event,proto3,enum=cluster.services.api.ClusterEvent" json:"event,omitempty"`
	WrappedMessage *anypb.Any   `protobuf:"bytes,2,opt,name=wrappedMessage,proto3" json:"wrappedMessage,omitempty"`
}

func (x *ClusterMessage) Reset() {
	*x = ClusterMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClusterMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterMessage) ProtoMessage() {}

func (x *ClusterMessage) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClusterMessage.ProtoReflect.Descriptor instead.
func (*ClusterMessage) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{0}
}

func (x *ClusterMessage) GetEvent() ClusterEvent {
	if x != nil {
		return x.Event
	}
	return ClusterEvent_ERROR
}

func (x *ClusterMessage) GetWrappedMessage() *anypb.Any {
	if x != nil {
		return x.WrappedMessage
	}
	return nil
}

type ErrorResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Error string `protobuf:"bytes,1,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *ErrorResponse) Reset() {
	*x = ErrorResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ErrorResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrorResponse) ProtoMessage() {}

func (x *ErrorResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorResponse.ProtoReflect.Descriptor instead.
func (*ErrorResponse) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{1}
}

func (x *ErrorResponse) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type VmSpawnRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cores    uint32 `protobuf:"varint,1,opt,name=cores,proto3" json:"cores,omitempty"`
	Memory   uint32 `protobuf:"varint,2,opt,name=memory,proto3" json:"memory,omitempty"`
	ImageRef string `protobuf:"bytes,3,opt,name=image_ref,json=imageRef,proto3" json:"image_ref,omitempty"`
	DryRun   bool   `protobuf:"varint,4,opt,name=dry_run,json=dryRun,proto3" json:"dry_run,omitempty"`
}

func (x *VmSpawnRequest) Reset() {
	*x = VmSpawnRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VmSpawnRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VmSpawnRequest) ProtoMessage() {}

func (x *VmSpawnRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VmSpawnRequest.ProtoReflect.Descriptor instead.
func (*VmSpawnRequest) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{2}
}

func (x *VmSpawnRequest) GetCores() uint32 {
	if x != nil {
		return x.Cores
	}
	return 0
}

func (x *VmSpawnRequest) GetMemory() uint32 {
	if x != nil {
		return x.Memory
	}
	return 0
}

func (x *VmSpawnRequest) GetImageRef() string {
	if x != nil {
		return x.ImageRef
	}
	return ""
}

func (x *VmSpawnRequest) GetDryRun() bool {
	if x != nil {
		return x.DryRun
	}
	return false
}

type VmSpawnResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id   string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Port uint32 `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
}

func (x *VmSpawnResponse) Reset() {
	*x = VmSpawnResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VmSpawnResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VmSpawnResponse) ProtoMessage() {}

func (x *VmSpawnResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VmSpawnResponse.ProtoReflect.Descriptor instead.
func (*VmSpawnResponse) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{3}
}

func (x *VmSpawnResponse) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *VmSpawnResponse) GetPort() uint32 {
	if x != nil {
		return x.Port
	}
	return 0
}

type VmQueryRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *VmQueryRequest) Reset() {
	*x = VmQueryRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VmQueryRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VmQueryRequest) ProtoMessage() {}

func (x *VmQueryRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VmQueryRequest.ProtoReflect.Descriptor instead.
func (*VmQueryRequest) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{4}
}

type VmQueryResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Vms map[string]*VmSpawnRequest `protobuf:"bytes,1,rep,name=vms,proto3" json:"vms,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *VmQueryResponse) Reset() {
	*x = VmQueryResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_cluster_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VmQueryResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VmQueryResponse) ProtoMessage() {}

func (x *VmQueryResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_cluster_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VmQueryResponse.ProtoReflect.Descriptor instead.
func (*VmQueryResponse) Descriptor() ([]byte, []int) {
	return file_pkg_proto_cluster_proto_rawDescGZIP(), []int{5}
}

func (x *VmQueryResponse) GetVms() map[string]*VmSpawnRequest {
	if x != nil {
		return x.Vms
	}
	return nil
}

var File_pkg_proto_cluster_proto protoreflect.FileDescriptor

var file_pkg_proto_cluster_proto_rawDesc = []byte{
	0x0a, 0x17, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x63, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x61, 0x70, 0x69, 0x1a,
	0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x88, 0x01, 0x0a, 0x0e, 0x43,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x38, 0x0a,
	0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x22, 0x2e, 0x63,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e,
	0x61, 0x70, 0x69, 0x2e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x45, 0x76, 0x65, 0x6e, 0x74,
	0x52, 0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x3c, 0x0a, 0x0e, 0x77, 0x72, 0x61, 0x70, 0x70,
	0x65, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x0e, 0x77, 0x72, 0x61, 0x70, 0x70, 0x65, 0x64, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0x25, 0x0a, 0x0d, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x22, 0x74, 0x0a, 0x0e,
	0x56, 0x6d, 0x53, 0x70, 0x61, 0x77, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14,
	0x0a, 0x05, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x63,
	0x6f, 0x72, 0x65, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0d, 0x52, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x12, 0x1b, 0x0a, 0x09,
	0x69, 0x6d, 0x61, 0x67, 0x65, 0x5f, 0x72, 0x65, 0x66, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x52, 0x65, 0x66, 0x12, 0x17, 0x0a, 0x07, 0x64, 0x72, 0x79,
	0x5f, 0x72, 0x75, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x64, 0x72, 0x79, 0x52,
	0x75, 0x6e, 0x22, 0x35, 0x0a, 0x0f, 0x56, 0x6d, 0x53, 0x70, 0x61, 0x77, 0x6e, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0d, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x22, 0x10, 0x0a, 0x0e, 0x56, 0x6d, 0x51,
	0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0xb1, 0x01, 0x0a, 0x0f,
	0x56, 0x6d, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x40, 0x0a, 0x03, 0x76, 0x6d, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x63,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e,
	0x61, 0x70, 0x69, 0x2e, 0x56, 0x6d, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x2e, 0x56, 0x6d, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x03, 0x76, 0x6d,
	0x73, 0x1a, 0x5c, 0x0a, 0x08, 0x56, 0x6d, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a,
	0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12,
	0x3a, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24,
	0x2e, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x73, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x56, 0x6d, 0x53, 0x70, 0x61, 0x77, 0x6e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x2a,
	0x24, 0x0a, 0x0c, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12,
	0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x53, 0x50,
	0x41, 0x57, 0x4e, 0x10, 0x01, 0x32, 0x66, 0x0a, 0x0e, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x54, 0x0a, 0x05, 0x53, 0x70, 0x61, 0x77, 0x6e,
	0x12, 0x24, 0x2e, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x73, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x56, 0x6d, 0x53, 0x70, 0x61, 0x77, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x25, 0x2e, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x56, 0x6d,
	0x53, 0x70, 0x61, 0x77, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x1b, 0x5a,
	0x19, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x3b, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_pkg_proto_cluster_proto_rawDescOnce sync.Once
	file_pkg_proto_cluster_proto_rawDescData = file_pkg_proto_cluster_proto_rawDesc
)

func file_pkg_proto_cluster_proto_rawDescGZIP() []byte {
	file_pkg_proto_cluster_proto_rawDescOnce.Do(func() {
		file_pkg_proto_cluster_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_cluster_proto_rawDescData)
	})
	return file_pkg_proto_cluster_proto_rawDescData
}

var file_pkg_proto_cluster_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pkg_proto_cluster_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_pkg_proto_cluster_proto_goTypes = []any{
	(ClusterEvent)(0),       // 0: cluster.services.api.ClusterEvent
	(*ClusterMessage)(nil),  // 1: cluster.services.api.ClusterMessage
	(*ErrorResponse)(nil),   // 2: cluster.services.api.ErrorResponse
	(*VmSpawnRequest)(nil),  // 3: cluster.services.api.VmSpawnRequest
	(*VmSpawnResponse)(nil), // 4: cluster.services.api.VmSpawnResponse
	(*VmQueryRequest)(nil),  // 5: cluster.services.api.VmQueryRequest
	(*VmQueryResponse)(nil), // 6: cluster.services.api.VmQueryResponse
	nil,                     // 7: cluster.services.api.VmQueryResponse.VmsEntry
	(*anypb.Any)(nil),       // 8: google.protobuf.Any
}
var file_pkg_proto_cluster_proto_depIdxs = []int32{
	0, // 0: cluster.services.api.ClusterMessage.event:type_name -> cluster.services.api.ClusterEvent
	8, // 1: cluster.services.api.ClusterMessage.wrappedMessage:type_name -> google.protobuf.Any
	7, // 2: cluster.services.api.VmQueryResponse.vms:type_name -> cluster.services.api.VmQueryResponse.VmsEntry
	3, // 3: cluster.services.api.VmQueryResponse.VmsEntry.value:type_name -> cluster.services.api.VmSpawnRequest
	3, // 4: cluster.services.api.ClusterService.Spawn:input_type -> cluster.services.api.VmSpawnRequest
	4, // 5: cluster.services.api.ClusterService.Spawn:output_type -> cluster.services.api.VmSpawnResponse
	5, // [5:6] is the sub-list for method output_type
	4, // [4:5] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_pkg_proto_cluster_proto_init() }
func file_pkg_proto_cluster_proto_init() {
	if File_pkg_proto_cluster_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_cluster_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*ClusterMessage); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_cluster_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*ErrorResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_cluster_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*VmSpawnRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_cluster_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*VmSpawnResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_cluster_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*VmQueryRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_cluster_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*VmQueryResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_proto_cluster_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_proto_cluster_proto_goTypes,
		DependencyIndexes: file_pkg_proto_cluster_proto_depIdxs,
		EnumInfos:         file_pkg_proto_cluster_proto_enumTypes,
		MessageInfos:      file_pkg_proto_cluster_proto_msgTypes,
	}.Build()
	File_pkg_proto_cluster_proto = out.File
	file_pkg_proto_cluster_proto_rawDesc = nil
	file_pkg_proto_cluster_proto_goTypes = nil
	file_pkg_proto_cluster_proto_depIdxs = nil
}
