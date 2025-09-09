// libhypercuda.c - Hardened CUDA shim for GPU quota enforcement
#define _GNU_SOURCE
#include <dlfcn.h>
#include <cuda.h>
#include <cuda_runtime_api.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// Configuration
static size_t VRAM_LIMIT = 3ULL * 1024ULL * 1024ULL * 1024ULL; // 3 GiB default
static int prefetch = 0;
static int device_id = 0;
static int shim_disabled = 0;
static int quota_violations = 0;

// Thread-safe allocation tracking
static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static size_t allocated = 0;

// Pointer tracking map
typedef struct Node { 
    void* ptr; 
    size_t size; 
    struct Node* next; 
} Node;
static Node* allocation_map = NULL;

// Function pointers for real CUDA functions
static cudaError_t (*real_cudaMallocManaged)(void**, size_t, unsigned int) = NULL;
static cudaError_t (*real_cudaFree)(void*) = NULL;
static cudaError_t (*real_cudaMalloc)(void**, size_t) = NULL;
static cudaError_t (*real_cudaFreeAsync)(void*, cudaStream_t) = NULL;
static cudaError_t (*real_cudaMallocAsync)(void**, size_t, cudaStream_t) = NULL;
static cudaError_t (*real_cudaMemGetInfo)(size_t*, size_t*) = NULL;
static cudaError_t (*real_cudaMemPrefetchAsync)(void*, size_t, int, cudaStream_t) = NULL;

// CUDA Driver API function pointers
static CUresult (*real_cuMemAlloc)(CUdeviceptr*, size_t) = NULL;
static CUresult (*real_cuMemAllocManaged)(CUdeviceptr*, size_t, unsigned int) = NULL;
static CUresult (*real_cuMemFree)(CUdeviceptr) = NULL;
static CUresult (*real_cuMemGetInfo)(size_t*, size_t*) = NULL;

__attribute__((constructor))
static void init(void) {
    // Check if shim is disabled
    if (getenv("HYPERCORE_DISABLE_SHIM")) {
        shim_disabled = 1;
        fprintf(stderr, "[hypercore] CUDA shim disabled via HYPERCORE_DISABLE_SHIM\n");
        return;
    }
    
    // Read configuration from environment
    const char* vram_limit = getenv("HYPERCORE_VRAM_LIMIT_BYTES");
    if (vram_limit && vram_limit[0]) {
        char *end = NULL;
        uint64_t v = strtoull(vram_limit, &end, 10);
        if (end && end != vram_limit && v > 0) {
            VRAM_LIMIT = (size_t)v;
        }
    }
    
    prefetch = getenv("HYPERCORE_PREFETCH") ? 1 : 0;
    cudaGetDevice(&device_id);
    
    // Resolve real function pointers
    real_cudaMallocManaged = dlsym(RTLD_NEXT, "cudaMallocManaged");
    real_cudaFree = dlsym(RTLD_NEXT, "cudaFree");
    real_cudaMalloc = dlsym(RTLD_NEXT, "cudaMalloc");
    real_cudaFreeAsync = dlsym(RTLD_NEXT, "cudaFreeAsync");
    real_cudaMallocAsync = dlsym(RTLD_NEXT, "cudaMallocAsync");
    real_cudaMemGetInfo = dlsym(RTLD_NEXT, "cudaMemGetInfo");
    real_cudaMemPrefetchAsync = dlsym(RTLD_NEXT, "cudaMemPrefetchAsync");
    
    // CUDA Driver API
    real_cuMemAlloc = dlsym(RTLD_NEXT, "cuMemAlloc_v2");
    real_cuMemAllocManaged = dlsym(RTLD_NEXT, "cuMemAllocManaged");
    real_cuMemFree = dlsym(RTLD_NEXT, "cuMemFree_v2");
    real_cuMemGetInfo = dlsym(RTLD_NEXT, "cuMemGetInfo_v2");
    
    fprintf(stderr, "[hypercore] CUDA shim initialized: limit=%zu bytes, prefetch=%d, device=%d\n", 
            VRAM_LIMIT, prefetch, device_id);
}

// Thread-safe allocation map operations
static void map_put(void* ptr, size_t size) {
    Node* node = (Node*)malloc(sizeof(Node));
    node->ptr = ptr;
    node->size = size;
    
    pthread_mutex_lock(&mu);
    node->next = allocation_map;
    allocation_map = node;
    allocated += size;
    pthread_mutex_unlock(&mu);
}

static size_t map_del(void* ptr) {
    pthread_mutex_lock(&mu);
    Node **cur = &allocation_map;
    Node *prev = NULL;
    
    while (*cur) {
        if ((*cur)->ptr == ptr) {
            Node* node = *cur;
            size_t size = node->size;
            *cur = node->next;
            free(node);
            allocated -= size;
            pthread_mutex_unlock(&mu);
            return size;
        }
        prev = *cur;
        cur = &((*cur)->next);
    }
    pthread_mutex_unlock(&mu);
    return 0;
}

// Quota check
static int allow_allocation(size_t size) {
    pthread_mutex_lock(&mu);
    size_t left = (allocated < VRAM_LIMIT) ? (VRAM_LIMIT - allocated) : 0;
    pthread_mutex_unlock(&mu);
    
    if (size > left) {
        quota_violations++;
        fprintf(stderr, "[hypercore] quota exceeded: want=%zu, left=%zu, violations=%d\n", 
                size, left, quota_violations);
        return 0;
    }
    return 1;
}

// Optional prefetch for performance
static void maybe_prefetch(void* ptr, size_t size) {
    if (!prefetch || !real_cudaMemPrefetchAsync) return;
    real_cudaMemPrefetchAsync(ptr, size, device_id, 0); // best-effort
}

// CUDA Runtime API implementations
cudaError_t cudaMalloc(void **devPtr, size_t size) {
    if (shim_disabled) {
        return real_cudaMalloc ? real_cudaMalloc(devPtr, size) : cudaErrorUnknown;
    }
    
    if (!devPtr) return cudaErrorInvalidValue;
    if (!allow_allocation(size)) return cudaErrorMemoryAllocation;
    
    cudaError_t err = real_cudaMallocManaged ? 
        real_cudaMallocManaged(devPtr, size, 1) : cudaErrorUnknown;
    
    if (err == cudaSuccess) {
        map_put(*devPtr, size);
        maybe_prefetch(*devPtr, size);
    }
    
    return err;
}

cudaError_t cudaFree(void *devPtr) {
    if (shim_disabled) {
        return real_cudaFree ? real_cudaFree(devPtr) : cudaErrorUnknown;
    }
    
    map_del(devPtr);
    return real_cudaFree ? real_cudaFree(devPtr) : cudaErrorUnknown;
}

cudaError_t cudaMallocAsync(void **devPtr, size_t size, cudaStream_t stream) {
    if (shim_disabled) {
        return real_cudaMallocAsync ? real_cudaMallocAsync(devPtr, size, stream) : cudaErrorUnknown;
    }
    
    if (!devPtr) return cudaErrorInvalidValue;
    if (!allow_allocation(size)) return cudaErrorMemoryAllocation;
    
    // Force managed memory for consistent behavior
    cudaError_t err = real_cudaMallocManaged ? 
        real_cudaMallocManaged(devPtr, size, 1) : cudaErrorUnknown;
    
    if (err == cudaSuccess) {
        map_put(*devPtr, size);
        maybe_prefetch(*devPtr, size);
    }
    
    return err;
}

cudaError_t cudaFreeAsync(void *devPtr, cudaStream_t stream) {
    if (shim_disabled) {
        return real_cudaFreeAsync ? real_cudaFreeAsync(devPtr, stream) : cudaErrorUnknown;
    }
    
    map_del(devPtr);
    return real_cudaFreeAsync ? real_cudaFreeAsync(devPtr, stream) : cudaSuccess;
}

cudaError_t cudaMemGetInfo(size_t *freeMem, size_t *totalMem) {
    if (shim_disabled) {
        return real_cudaMemGetInfo ? real_cudaMemGetInfo(freeMem, totalMem) : cudaErrorUnknown;
    }
    
    if (!freeMem || !totalMem) return cudaErrorInvalidValue;
    
    pthread_mutex_lock(&mu);
    *totalMem = VRAM_LIMIT;
    *freeMem = (allocated < VRAM_LIMIT) ? (VRAM_LIMIT - allocated) : 0;
    pthread_mutex_unlock(&mu);
    
    return cudaSuccess;
}

// CUDA Driver API implementations
CUresult cuMemAlloc_v2(CUdeviceptr *dptr, size_t size) {
    if (shim_disabled) {
        return real_cuMemAlloc ? real_cuMemAlloc(dptr, size) : CUDA_ERROR_NOT_INITIALIZED;
    }
    
    if (!allow_allocation(size)) return CUDA_ERROR_OUT_OF_MEMORY;
    
    CUresult r = real_cuMemAllocManaged ? 
        real_cuMemAllocManaged(dptr, size, 1) : CUDA_ERROR_NOT_INITIALIZED;
    
    if (r == CUDA_SUCCESS) {
        map_put((void*)(uintptr_t)(*dptr), size);
    }
    
    return r;
}

CUresult cuMemFree_v2(CUdeviceptr dptr) {
    if (shim_disabled) {
        return real_cuMemFree ? real_cuMemFree(dptr) : CUDA_ERROR_NOT_INITIALIZED;
    }
    
    map_del((void*)(uintptr_t)dptr);
    return real_cuMemFree ? real_cuMemFree(dptr) : CUDA_ERROR_NOT_INITIALIZED;
}

CUresult cuMemGetInfo_v2(size_t *freeMem, size_t *totalMem) {
    if (shim_disabled) {
        return real_cuMemGetInfo ? real_cuMemGetInfo(freeMem, totalMem) : CUDA_ERROR_NOT_INITIALIZED;
    }
    
    if (!freeMem || !totalMem) return CUDA_ERROR_INVALID_VALUE;
    
    pthread_mutex_lock(&mu);
    *totalMem = VRAM_LIMIT;
    *freeMem = (allocated < VRAM_LIMIT) ? (VRAM_LIMIT - allocated) : 0;
    pthread_mutex_unlock(&mu);
    
    return CUDA_SUCCESS;
}

// Additional CUDA functions
cudaError_t cudaMallocHost(void **ptr, size_t size) {
    // Host memory doesn't count against GPU quota
    return real_cudaMalloc ? real_cudaMalloc(ptr, size) : cudaErrorUnknown;
}

cudaError_t cudaMallocPitch(void **devPtr, size_t *pitch, size_t width, size_t height) {
    size_t size = width * height;
    return cudaMalloc(devPtr, size);
}

// Utility function to get current allocation info
void hypercore_get_allocation_info(size_t *allocated_bytes, size_t *limit_bytes, int *violations) {
    pthread_mutex_lock(&mu);
    if (allocated_bytes) *allocated_bytes = allocated;
    if (limit_bytes) *limit_bytes = VRAM_LIMIT;
    if (violations) *violations = quota_violations;
    pthread_mutex_unlock(&mu);
}