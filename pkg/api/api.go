package api

import (
	"encoding/json"
	"net/http"
)

func StartVMHandler(w http.ResponseWriter, r *http.Request) {
	// var config hypervisor.VMConfig

	// if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// For now, use Firecracker as the hypervisor. This can be made dynamic.
	// hv := firecracker.NewFirecracker()
	// vmID, err := hv.StartVM(config)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"vm_id": "vmID"})
}

// SetupRoutes sets up the API routes.
func SetupRoutes() {
	http.HandleFunc("/start-vm", StartVMHandler)
	http.ListenAndServe(":8080", nil)
}
