// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v5.27.1
// source: pkg/api/services/vms.proto

package microvm

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// VMServiceClient is the client API for VMService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type VMServiceClient interface {
	Create(ctx context.Context, in *CreateMicroVMRequest, opts ...grpc.CallOption) (*CreateMicroVMResponse, error)
	Delete(ctx context.Context, in *DeleteMicroVMRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	Get(ctx context.Context, in *GetMicroVMRequest, opts ...grpc.CallOption) (*GetMicroVMResponse, error)
	List(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ListMicroVMsResponse, error)
}

type vMServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewVMServiceClient(cc grpc.ClientConnInterface) VMServiceClient {
	return &vMServiceClient{cc}
}

func (c *vMServiceClient) Create(ctx context.Context, in *CreateMicroVMRequest, opts ...grpc.CallOption) (*CreateMicroVMResponse, error) {
	out := new(CreateMicroVMResponse)
	err := c.cc.Invoke(ctx, "/vm.services.api.VMService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vMServiceClient) Delete(ctx context.Context, in *DeleteMicroVMRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/vm.services.api.VMService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vMServiceClient) Get(ctx context.Context, in *GetMicroVMRequest, opts ...grpc.CallOption) (*GetMicroVMResponse, error) {
	out := new(GetMicroVMResponse)
	err := c.cc.Invoke(ctx, "/vm.services.api.VMService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *vMServiceClient) List(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ListMicroVMsResponse, error) {
	out := new(ListMicroVMsResponse)
	err := c.cc.Invoke(ctx, "/vm.services.api.VMService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// VMServiceServer is the server API for VMService service.
// All implementations should embed UnimplementedVMServiceServer
// for forward compatibility
type VMServiceServer interface {
	Create(context.Context, *CreateMicroVMRequest) (*CreateMicroVMResponse, error)
	Delete(context.Context, *DeleteMicroVMRequest) (*emptypb.Empty, error)
	Get(context.Context, *GetMicroVMRequest) (*GetMicroVMResponse, error)
	List(context.Context, *emptypb.Empty) (*ListMicroVMsResponse, error)
}

// UnimplementedVMServiceServer should be embedded to have forward compatible implementations.
type UnimplementedVMServiceServer struct {
}

func (UnimplementedVMServiceServer) Create(context.Context, *CreateMicroVMRequest) (*CreateMicroVMResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedVMServiceServer) Delete(context.Context, *DeleteMicroVMRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedVMServiceServer) Get(context.Context, *GetMicroVMRequest) (*GetMicroVMResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedVMServiceServer) List(context.Context, *emptypb.Empty) (*ListMicroVMsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}

// UnsafeVMServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to VMServiceServer will
// result in compilation errors.
type UnsafeVMServiceServer interface {
	mustEmbedUnimplementedVMServiceServer()
}

func RegisterVMServiceServer(s grpc.ServiceRegistrar, srv VMServiceServer) {
	s.RegisterService(&VMService_ServiceDesc, srv)
}

func _VMService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateMicroVMRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VMServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/vm.services.api.VMService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VMServiceServer).Create(ctx, req.(*CreateMicroVMRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VMService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteMicroVMRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VMServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/vm.services.api.VMService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VMServiceServer).Delete(ctx, req.(*DeleteMicroVMRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VMService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetMicroVMRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VMServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/vm.services.api.VMService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VMServiceServer).Get(ctx, req.(*GetMicroVMRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _VMService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(VMServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/vm.services.api.VMService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(VMServiceServer).List(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

// VMService_ServiceDesc is the grpc.ServiceDesc for VMService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var VMService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "vm.services.api.VMService",
	HandlerType: (*VMServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _VMService_Create_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _VMService_Delete_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _VMService_Get_Handler,
		},
		{
			MethodName: "List",
			Handler:    _VMService_List_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/api/services/vms.proto",
}
