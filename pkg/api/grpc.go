package api

import (
	"context"
	"fmt"
	vm "vistara-node/pkg/api/services/microvm"
	"vistara-node/pkg/api/types"
	"vistara-node/pkg/app"
	"vistara-node/pkg/defaults"
	"vistara-node/pkg/log"
	"vistara-node/pkg/models"
	"vistara-node/pkg/ports"

	"google.golang.org/protobuf/types/known/emptypb"
)

func NewServer(commandSvc *app.App) ports.MicroVMGRPCService {
	return &server{
		commandSvc: commandSvc,
	}
}

type server struct {
	commandSvc *app.App
}

// This is the Create request: vm.services.api.VMService Create(), similar to CreateMicroVM
func (s *server) Create(
	ctx context.Context, req *vm.CreateMicroVMRequest,
) (*vm.CreateMicroVMResponse, error) {
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

	resp := &vm.CreateMicroVMResponse{
		Microvm: &types.MicroVM{
			Version: int32(createdVm.Version),
			Spec:    convertModelToMicroVMSpec(createdVm),
		},
	}

	return resp, nil
}

func (s *server) Delete(
	ctx context.Context, req *vm.DeleteMicroVMRequest,
) (*emptypb.Empty, error) {
	logger := log.GetLogger(ctx)
	logger.Infof("Deleting microvm %v", req)

	vmid, err := models.NewVMID(req.Id, defaults.MicroVMNamespace, "0")
	if err != nil {
		return nil, fmt.Errorf("creating vmid: %w", err)
	}

	err = s.commandSvc.Delete(ctx, *vmid)
	if err != nil {
		return nil, fmt.Errorf("deleting microvm: %w", err)
	}

	return nil, nil
}

// Get implements ports.MicroVMGRPCService.
func (s *server) Get(ctx context.Context, req *vm.GetMicroVMRequest) (*vm.GetMicroVMResponse, error) {
	logger := log.GetLogger(ctx)
	logger.Info("Getting microvm %v", req)
	return nil, nil
}

// List implements ports.MicroVMGRPCService.
func (s *server) List(ctx context.Context, req *vm.ListMicroVMsRequest) (*vm.ListMicroVMsResponse, error) {
	panic("unimplemented")
}

// ListVMsStream implements ports.MicroVMGRPCService.
func (s *server) ListVMsStream(ctx *vm.ListMicroVMsRequest, req vm.VMService_ListVMsStreamServer) error {
	panic("unimplemented")
}

// mustEmbedUnimplementedVMServiceServer implements ports.MicroVMGRPCService.
func mustEmbedUnimplementedVMServiceServer() {
	panic("unimplemented")
}
