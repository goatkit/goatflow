//go:build linux && (amd64 || arm64)

package main

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// Offsets into struct seccomp_data.
const (
	offNr   = 0
	offArch = 4
	offArg0 = 16 // low 32 bits on little-endian
)

const (
	bpfLoad = unix.BPF_LD | unix.BPF_W | unix.BPF_ABS
	bpfJEq  = unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K
	bpfJGE  = unix.BPF_JMP | unix.BPF_JGE | unix.BPF_K
	bpfRet  = unix.BPF_RET | unix.BPF_K

	retEPERM = unix.SECCOMP_RET_ERRNO | uint32(unix.EPERM)
	x32Bit   = 0x40000000
)

// networkFilter allows socket() only for AF_UNIX and refuses io_uring_setup
// (and, on amd64, every x32 syscall) with EPERM. Any other architecture than
// the one this binary was built for kills the process.
func networkFilter() []unix.SockFilter {
	f := make([]unix.SockFilter, 0, 12)
	f = append(f,
		unix.SockFilter{Code: bpfLoad, K: offArch},
		unix.SockFilter{Code: bpfJEq, Jt: 1, K: auditArch},
		unix.SockFilter{Code: bpfRet, K: unix.SECCOMP_RET_KILL_PROCESS},
		unix.SockFilter{Code: bpfLoad, K: offNr},
	)
	if denyX32 {
		f = append(f, unix.SockFilter{Code: bpfJGE, Jt: 6, K: x32Bit}) // -> deny
	}
	return append(f,
		unix.SockFilter{Code: bpfJEq, Jt: 5, K: unix.SYS_IO_URING_SETUP}, // -> deny
		unix.SockFilter{Code: bpfJEq, Jt: 1, K: unix.SYS_SOCKET},         // -> family check
		unix.SockFilter{Code: bpfRet, K: unix.SECCOMP_RET_ALLOW},
		unix.SockFilter{Code: bpfLoad, K: offArg0},
		unix.SockFilter{Code: bpfJEq, Jf: 1, K: unix.AF_UNIX},
		unix.SockFilter{Code: bpfRet, K: unix.SECCOMP_RET_ALLOW},
		unix.SockFilter{Code: bpfRet, K: retEPERM}, // deny
	)
}

// denyNetwork installs networkFilter on every thread of this process; it
// survives exec.
func denyNetwork() error {
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return err
	}
	f := networkFilter()
	prog := unix.SockFprog{Len: uint16(len(f)), Filter: &f[0]} // #nosec G115 -- fixed-size filter
	// #nosec G103 -- seccomp(2) takes a pointer to the filter program
	r, _, errno := unix.Syscall(unix.SYS_SECCOMP, unix.SECCOMP_SET_MODE_FILTER,
		unix.SECCOMP_FILTER_FLAG_TSYNC, uintptr(unsafe.Pointer(&prog)))
	if errno != 0 {
		return errno
	}
	if r != 0 {
		return unix.Errno(unix.ESRCH)
	}
	return nil
}
