// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.25.3
// source: pkg/api/types/microvm.proto

package types

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// MicroVM represents a microvm machine that is created via a provider.
type MicroVM struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version int32 `protobuf:"varint,1,opt,name=version,proto3" json:"version,omitempty"`
	// Spec is the specification of the microvm.
	Spec *MicroVMSpec `protobuf:"bytes,2,opt,name=spec,proto3" json:"spec,omitempty"`
}

func (x *MicroVM) Reset() {
	*x = MicroVM{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_api_types_microvm_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MicroVM) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MicroVM) ProtoMessage() {}

func (x *MicroVM) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_api_types_microvm_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MicroVM.ProtoReflect.Descriptor instead.
func (*MicroVM) Descriptor() ([]byte, []int) {
	return file_pkg_api_types_microvm_proto_rawDescGZIP(), []int{0}
}

func (x *MicroVM) GetVersion() int32 {
	if x != nil {
		return x.Version
	}
	return 0
}

func (x *MicroVM) GetSpec() *MicroVMSpec {
	if x != nil {
		return x.Spec
	}
	return nil
}

// MicroVMSpec represents the specification for a microvm.
type MicroVMSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ID is the identifier of the microvm.
	// If this empty at creation time a ID will be automatically generated.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// VCPU specifies how many vcpu the machine will be allocated.
	Vcpu int32 `protobuf:"varint,2,opt,name=vcpu,proto3" json:"vcpu,omitempty"`
	// MemoryInMb is the amount of memory in megabytes that the machine will be allocated.
	MemoryInMb int32 `protobuf:"varint,3,opt,name=memory_in_mb,json=memoryInMb,proto3" json:"memory_in_mb,omitempty"`
	// Kernel is the details of the kernel to use .
	KernelPath string `protobuf:"bytes,4,opt,name=kernel_path,json=kernelPath,proto3" json:"kernel_path,omitempty"`
	// RootVolume specifies the root volume mount for the MicroVM.
	RootfsPath string `protobuf:"bytes,5,opt,name=rootfs_path,json=rootfsPath,proto3" json:"rootfs_path,omitempty"`
	// HostNetDev is the device to use for passing traffic through the TAP device
	HostNetDev string `protobuf:"bytes,6,opt,name=HostNetDev,proto3" json:"HostNetDev,omitempty"`
	// MAC address of the guest interface
	GuestMac string `protobuf:"bytes,7,opt,name=GuestMac,proto3" json:"GuestMac,omitempty"`
	// CreatedAt indicates the time the microvm was created at.
	CreatedAt *timestamppb.Timestamp `protobuf:"bytes,12,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	// UpdatedAt indicates the time the microvm was last updated.
	UpdatedAt *timestamppb.Timestamp `protobuf:"bytes,13,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	// DeletedAt indicates the time the microvm was marked as deleted.
	DeletedAt *timestamppb.Timestamp `protobuf:"bytes,14,opt,name=deleted_at,json=deletedAt,proto3" json:"deleted_at,omitempty"`
	// Provider allows you to specify the name of the microvm provider to use. If this isn't supplied
	// then the default provider will be used.
	Provider *string `protobuf:"bytes,16,opt,name=provider,proto3,oneof" json:"provider,omitempty"`
}

func (x *MicroVMSpec) Reset() {
	*x = MicroVMSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_api_types_microvm_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MicroVMSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MicroVMSpec) ProtoMessage() {}

func (x *MicroVMSpec) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_api_types_microvm_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MicroVMSpec.ProtoReflect.Descriptor instead.
func (*MicroVMSpec) Descriptor() ([]byte, []int) {
	return file_pkg_api_types_microvm_proto_rawDescGZIP(), []int{1}
}

func (x *MicroVMSpec) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *MicroVMSpec) GetVcpu() int32 {
	if x != nil {
		return x.Vcpu
	}
	return 0
}

func (x *MicroVMSpec) GetMemoryInMb() int32 {
	if x != nil {
		return x.MemoryInMb
	}
	return 0
}

func (x *MicroVMSpec) GetKernelPath() string {
	if x != nil {
		return x.KernelPath
	}
	return ""
}

func (x *MicroVMSpec) GetRootfsPath() string {
	if x != nil {
		return x.RootfsPath
	}
	return ""
}

func (x *MicroVMSpec) GetHostNetDev() string {
	if x != nil {
		return x.HostNetDev
	}
	return ""
}

func (x *MicroVMSpec) GetGuestMac() string {
	if x != nil {
		return x.GuestMac
	}
	return ""
}

func (x *MicroVMSpec) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

func (x *MicroVMSpec) GetUpdatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.UpdatedAt
	}
	return nil
}

func (x *MicroVMSpec) GetDeletedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.DeletedAt
	}
	return nil
}

func (x *MicroVMSpec) GetProvider() string {
	if x != nil && x.Provider != nil {
		return *x.Provider
	}
	return ""
}

var File_pkg_api_types_microvm_proto protoreflect.FileDescriptor

var file_pkg_api_types_microvm_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f,
	0x6d, 0x69, 0x63, 0x72, 0x6f, 0x76, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x45,
	0x0a, 0x07, 0x4d, 0x69, 0x63, 0x72, 0x6f, 0x56, 0x4d, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x20, 0x0a, 0x04, 0x73, 0x70, 0x65, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x0c, 0x2e, 0x4d, 0x69, 0x63, 0x72, 0x6f, 0x56, 0x4d, 0x53, 0x70, 0x65, 0x63, 0x52,
	0x04, 0x73, 0x70, 0x65, 0x63, 0x22, 0xb0, 0x03, 0x0a, 0x0b, 0x4d, 0x69, 0x63, 0x72, 0x6f, 0x56,
	0x4d, 0x53, 0x70, 0x65, 0x63, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x76, 0x63, 0x70, 0x75, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x04, 0x76, 0x63, 0x70, 0x75, 0x12, 0x20, 0x0a, 0x0c, 0x6d, 0x65, 0x6d,
	0x6f, 0x72, 0x79, 0x5f, 0x69, 0x6e, 0x5f, 0x6d, 0x62, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x0a, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x49, 0x6e, 0x4d, 0x62, 0x12, 0x1f, 0x0a, 0x0b, 0x6b,
	0x65, 0x72, 0x6e, 0x65, 0x6c, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x50, 0x61, 0x74, 0x68, 0x12, 0x1f, 0x0a, 0x0b,
	0x72, 0x6f, 0x6f, 0x74, 0x66, 0x73, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x72, 0x6f, 0x6f, 0x74, 0x66, 0x73, 0x50, 0x61, 0x74, 0x68, 0x12, 0x1e, 0x0a,
	0x0a, 0x48, 0x6f, 0x73, 0x74, 0x4e, 0x65, 0x74, 0x44, 0x65, 0x76, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x48, 0x6f, 0x73, 0x74, 0x4e, 0x65, 0x74, 0x44, 0x65, 0x76, 0x12, 0x1a, 0x0a,
	0x08, 0x47, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x61, 0x63, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x47, 0x75, 0x65, 0x73, 0x74, 0x4d, 0x61, 0x63, 0x12, 0x39, 0x0a, 0x0a, 0x63, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x64, 0x41, 0x74, 0x12, 0x39, 0x0a, 0x0a, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x5f,
	0x61, 0x74, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12,
	0x39, 0x0a, 0x0a, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x0e, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52,
	0x09, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12, 0x1f, 0x0a, 0x08, 0x70, 0x72,
	0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x18, 0x10, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x08,
	0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x88, 0x01, 0x01, 0x42, 0x0b, 0x0a, 0x09, 0x5f,
	0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x42, 0x15, 0x5a, 0x13, 0x70, 0x6b, 0x67, 0x2f,
	0x61, 0x70, 0x69, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x3b, 0x74, 0x79, 0x70, 0x65, 0x73, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_api_types_microvm_proto_rawDescOnce sync.Once
	file_pkg_api_types_microvm_proto_rawDescData = file_pkg_api_types_microvm_proto_rawDesc
)

func file_pkg_api_types_microvm_proto_rawDescGZIP() []byte {
	file_pkg_api_types_microvm_proto_rawDescOnce.Do(func() {
		file_pkg_api_types_microvm_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_api_types_microvm_proto_rawDescData)
	})
	return file_pkg_api_types_microvm_proto_rawDescData
}

var file_pkg_api_types_microvm_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pkg_api_types_microvm_proto_goTypes = []interface{}{
	(*MicroVM)(nil),               // 0: MicroVM
	(*MicroVMSpec)(nil),           // 1: MicroVMSpec
	(*timestamppb.Timestamp)(nil), // 2: google.protobuf.Timestamp
}
var file_pkg_api_types_microvm_proto_depIdxs = []int32{
	1, // 0: MicroVM.spec:type_name -> MicroVMSpec
	2, // 1: MicroVMSpec.created_at:type_name -> google.protobuf.Timestamp
	2, // 2: MicroVMSpec.updated_at:type_name -> google.protobuf.Timestamp
	2, // 3: MicroVMSpec.deleted_at:type_name -> google.protobuf.Timestamp
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_pkg_api_types_microvm_proto_init() }
func file_pkg_api_types_microvm_proto_init() {
	if File_pkg_api_types_microvm_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_api_types_microvm_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MicroVM); i {
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
		file_pkg_api_types_microvm_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MicroVMSpec); i {
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
	file_pkg_api_types_microvm_proto_msgTypes[1].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_api_types_microvm_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_api_types_microvm_proto_goTypes,
		DependencyIndexes: file_pkg_api_types_microvm_proto_depIdxs,
		MessageInfos:      file_pkg_api_types_microvm_proto_msgTypes,
	}.Build()
	File_pkg_api_types_microvm_proto = out.File
	file_pkg_api_types_microvm_proto_rawDesc = nil
	file_pkg_api_types_microvm_proto_goTypes = nil
	file_pkg_api_types_microvm_proto_depIdxs = nil
}
