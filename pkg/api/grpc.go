package api

import (
	"context"
	"fmt"
	vm1 "vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/log"
	"vistara-node/pkg/ports"

	"google.golang.org/protobuf/types/known/emptypb"
)

func NewServer(commandSvc ports.MicroVMService) ports.MicroVMGRPCService {
	return &server{
		commandSvc: commandSvc,
	}
}

type server struct {
	commandSvc ports.MicroVMService
}

// This is the Create request: vm.services.api.VMService Create(), similar to CreateMicroVM
func (s *server) Create(
	ctx context.Context, req *vm1.CreateMicroVMRequest,
) (*vm1.CreateMicroVMResponse, error) {
	logger := log.GetLogger(ctx)
	logger.Debug("Converting request to model")

	if req == nil || req.Microvm == nil {
		return nil, fmt.Errorf("invalid request")
	}

	vmSpec, err := convertMicroVMToModel(req.Microvm)
	if err != nil {
		return nil, fmt.Errorf("converting request to model: %w", err)
	}

	createdVm, err := s.commandSvc.Create(ctx, vmSpec)
	if err != nil {
		return nil, fmt.Errorf("creating microvm: %w", err)
	}

	resp := &vm1.CreateMicroVMResponse{
		Microvm: &types.MicroVM{
			Version: int32(createdVm.Version),
			Spec:    convertModelToMicroVMSpec(createdVm),
		},
	}

	return resp, nil
}

func (s *server) Delete(
	ctx context.Context, req *vm1.DeleteMicroVMRequest,
) (*emptypb.Empty, error) {
	logger := log.GetLogger(ctx)
	logger.Info("Deleting microvm %v", req)

	// err := s.commandSvc.Delete(ctx, req.Id)
	// if err != nil {
	// 	return nil, fmt.Errorf("deleting microvm: %w", err)
	// }

	return nil, nil
}

// Get implements ports.MicroVMGRPCService.
func (s *server) Get(ctx context.Context, req *vm1.GetMicroVMRequest) (*vm1.GetMicroVMResponse, error) {
	logger := log.GetLogger(ctx)
	logger.Info("Getting microvm %v", req)
	return nil, nil
}

// List implements ports.MicroVMGRPCService.
func (s *server) List(ctx context.Context, req *vm1.ListMicroVMsRequest) (*vm1.ListMicroVMsResponse, error) {
	panic("unimplemented")
}

// ListVMsStream implements ports.MicroVMGRPCService.
func (s *server) ListVMsStream(ctx *vm1.ListMicroVMsRequest, req vm1.VMService_ListVMsStreamServer) error {
	panic("unimplemented")
}

// mustEmbedUnimplementedVMServiceServer implements ports.MicroVMGRPCService.
func mustEmbedUnimplementedVMServiceServer() {
	panic("unimplemented")
}
