package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"vistara/hypercore/pkg/scheduler"
	"vistara/hypercore/pkg/types"
)

// MIGAPI provides HTTP API for MIG operations
type MIGAPI struct {
	scheduler *scheduler.MIGScheduler
}

// NewMIGAPI creates a new MIG API handler
func NewMIGAPI() (*MIGAPI, error) {
	sched, err := scheduler.NewMIGScheduler()
	if err != nil {
		return nil, err
	}

	return &MIGAPI{
		scheduler: sched,
	}, nil
}

// AllocateGPURequest represents a GPU allocation request
type AllocateGPURequest struct {
	WorkloadID string         `json:"workload_id"`
	Profile    types.MIGProfile `json:"profile"`
	Timeout    int            `json:"timeout,omitempty"`
}

// AllocateGPUResponse represents a GPU allocation response
type AllocateGPUResponse struct {
	Allocation *types.AllocationInfo `json:"allocation"`
	Success    bool                  `json:"success"`
	Message    string                `json:"message"`
}

// DeviceStatusResponse represents device status information
type DeviceStatusResponse struct {
	Devices      []types.GPUDeviceInfo `json:"devices"`
	Utilization  map[string]float64    `json:"utilization"`
	Allocations  map[string]*types.AllocationInfo `json:"allocations"`
}

// RegisterRoutes registers all MIG API routes
func (api *MIGAPI) RegisterRoutes(router *mux.Router) {
	// GPU allocation endpoints
	router.HandleFunc("/api/v1/gpu/allocate", api.AllocateGPU).Methods("POST")
	router.HandleFunc("/api/v1/gpu/deallocate/{workload_id}", api.DeallocateGPU).Methods("DELETE")
	router.HandleFunc("/api/v1/gpu/allocation/{workload_id}", api.GetAllocation).Methods("GET")
	router.HandleFunc("/api/v1/gpu/allocations", api.GetAllAllocations).Methods("GET")

	// Device management endpoints
	router.HandleFunc("/api/v1/gpu/devices", api.GetDevices).Methods("GET")
	router.HandleFunc("/api/v1/gpu/devices/available", api.GetAvailableDevices).Methods("GET")
	router.HandleFunc("/api/v1/gpu/devices/utilization", api.GetDeviceUtilization).Methods("GET")
	router.HandleFunc("/api/v1/gpu/devices/status", api.GetDeviceStatus).Methods("GET")

	// Health check endpoint
	router.HandleFunc("/api/v1/health", api.HealthCheck).Methods("GET")
}

// AllocateGPU handles GPU allocation requests
func (api *MIGAPI) AllocateGPU(w http.ResponseWriter, r *http.Request) {
	var req AllocateGPURequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.WorkloadID == "" {
		http.Error(w, "workload_id is required", http.StatusBadRequest)
		return
	}

	allocation, err := api.scheduler.AllocateGPU(req.WorkloadID, req.Profile)
	if err != nil {
		response := AllocateGPUResponse{
			Success: false,
			Message: err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := AllocateGPUResponse{
		Allocation: allocation,
		Success:    true,
		Message:    "GPU allocated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeallocateGPU handles GPU deallocation requests
func (api *MIGAPI) DeallocateGPU(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workloadID := vars["workload_id"]

	if workloadID == "" {
		http.Error(w, "workload_id is required", http.StatusBadRequest)
		return
	}

	err := api.scheduler.DeallocateGPU(workloadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "GPU deallocated successfully for workload %s", workloadID)
}

// GetAllocation returns allocation information for a specific workload
func (api *MIGAPI) GetAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workloadID := vars["workload_id"]

	if workloadID == "" {
		http.Error(w, "workload_id is required", http.StatusBadRequest)
		return
	}

	allocation, err := api.scheduler.GetWorkloadAllocation(workloadID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allocation)
}

// GetAllAllocations returns all current allocations
func (api *MIGAPI) GetAllAllocations(w http.ResponseWriter, r *http.Request) {
	allocations := api.scheduler.GetAllAllocations()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(allocations)
}

// GetDevices returns all GPU devices
func (api *MIGAPI) GetDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := api.scheduler.GetAvailableDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// GetAvailableDevices returns available GPU devices
func (api *MIGAPI) GetAvailableDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := api.scheduler.GetAvailableDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// GetDeviceUtilization returns device utilization statistics
func (api *MIGAPI) GetDeviceUtilization(w http.ResponseWriter, r *http.Request) {
	utilization, err := api.scheduler.GetDeviceUtilization()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(utilization)
}

// GetDeviceStatus returns comprehensive device status
func (api *MIGAPI) GetDeviceStatus(w http.ResponseWriter, r *http.Request) {
	devices, err := api.scheduler.GetAvailableDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	utilization, err := api.scheduler.GetDeviceUtilization()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	allocations := api.scheduler.GetAllAllocations()

	response := DeviceStatusResponse{
		Devices:     devices,
		Utilization: utilization,
		Allocations: allocations,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthCheck provides health status
func (api *MIGAPI) HealthCheck(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "mig-api",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// CleanupExpiredAllocations handles cleanup of expired allocations
func (api *MIGAPI) CleanupExpiredAllocations(w http.ResponseWriter, r *http.Request) {
	timeoutStr := r.URL.Query().Get("timeout")
	timeout := 3600 // Default 1 hour

	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = t
		}
	}

	err := api.scheduler.CleanupExpiredAllocations(timeout)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Expired allocations cleaned up successfully")
}