#!/usr/bin/env python3
"""
FUSE-based /proc/meminfo replacement for CPU memory quota enforcement.
This makes Python libraries like psutil see a smaller memory limit.
"""

import os
import sys
import time
import stat
from fuse import FUSE, Operations

class MeminfoFS(Operations):
    def __init__(self, limit_mb=1024):
        self.limit_kb = limit_mb * 1024
        self.start_time = time.time()

    def getattr(self, path, fh=None):
        """Get file attributes for /proc/meminfo"""
        if path == '/meminfo':
            return dict(
                st_mode=(stat.S_IFREG | 0o444),
                st_nlink=1,
                st_size=1024,
                st_ctime=self.start_time,
                st_mtime=self.start_time,
                st_atime=self.start_time
            )
        else:
            raise OSError(2, "No such file or directory")

    def readdir(self, path, fh):
        """List directory contents"""
        if path == '/':
            return ['.', '..', 'meminfo']
        else:
            return []

    def open(self, path, flags):
        """Open file"""
        if path == '/meminfo':
            return 0
        else:
            raise OSError(2, "No such file or directory")

    def read(self, path, size, offset, fh):
        """Read /proc/meminfo content with fake memory limits"""
        if path != '/meminfo':
            return b''
        
        # Generate fake /proc/meminfo content
        # This makes psutil and other libraries see a smaller memory limit
        buf = (
            f"MemTotal:       {self.limit_kb} kB\n"
            f"MemFree:        {self.limit_kb} kB\n"
            f"MemAvailable:   {self.limit_kb} kB\n"
            f"Buffers:        0 kB\n"
            f"Cached:         0 kB\n"
            f"SwapCached:     0 kB\n"
            f"Active:         0 kB\n"
            f"Inactive:       0 kB\n"
            f"Active(anon):   0 kB\n"
            f"Inactive(anon): 0 kB\n"
            f"Active(file):   0 kB\n"
            f"Inactive(file): 0 kB\n"
            f"Unevictable:    0 kB\n"
            f"Mlocked:        0 kB\n"
            f"SwapTotal:      0 kB\n"
            f"SwapFree:       0 kB\n"
            f"Dirty:          0 kB\n"
            f"Writeback:      0 kB\n"
            f"AnonPages:      0 kB\n"
            f"Mapped:         0 kB\n"
            f"Shmem:          0 kB\n"
            f"Slab:           0 kB\n"
            f"SReclaimable:   0 kB\n"
            f"SUnreclaim:     0 kB\n"
            f"KernelStack:    0 kB\n"
            f"PageTables:     0 kB\n"
            f"NFS_Unstable:   0 kB\n"
            f"Bounce:         0 kB\n"
            f"WritebackTmp:   0 kB\n"
            f"CommitLimit:    {self.limit_kb} kB\n"
            f"Committed_AS:   0 kB\n"
            f"VmallocTotal:   0 kB\n"
            f"VmallocUsed:    0 kB\n"
            f"VmallocChunk:   0 kB\n"
            f"HardwareCorrupted: 0 kB\n"
            f"AnonHugePages:  0 kB\n"
            f"HugePages_Total: 0\n"
            f"HugePages_Free:  0\n"
            f"HugePages_Rsvd:  0\n"
            f"HugePages_Surp:  0\n"
            f"Hugepagesize:    2048 kB\n"
            f"DirectMap4k:     0 kB\n"
            f"DirectMap2M:     0 kB\n"
            f"DirectMap1G:     0 kB\n"
        )
        
        # Return the requested slice
        data = buf.encode('utf-8')
        return data[offset:offset+size]

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 meminfo_fuse.py <mountpoint> [limit_mb]", file=sys.stderr)
        sys.exit(1)
    
    mountpoint = sys.argv[1]
    limit_mb = int(sys.argv[2]) if len(sys.argv) > 2 else 2048
    
    print(f"Starting FUSE meminfo with {limit_mb}MB limit at {mountpoint}", file=sys.stderr)
    
    # Create mountpoint if it doesn't exist
    os.makedirs(mountpoint, exist_ok=True)
    
    # Start FUSE
    FUSE(
        MeminfoFS(limit_mb), 
        mountpoint, 
        nothreads=True, 
        foreground=True,
        allow_other=False
    )

if __name__ == '__main__':
    main()