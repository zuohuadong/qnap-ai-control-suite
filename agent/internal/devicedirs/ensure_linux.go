//go:build linux

package devicedirs

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
)

// 逐段打开目录描述符，拒绝所有符号链接；不做 chmod/chown/ACL 操作。
func openDir(parent int, name string) (int, error) {
	return syscall.Openat(parent, name, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
}

func Ensure(root string, request Request) (Receipt, error) {
	if err := request.Validate(); err != nil {
		return Receipt{}, err
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
		return Receipt{}, errors.New("invalid configured root")
	}
	fd, err := syscall.Open("/", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { syscall.Close(fd) }()
	// 租户和设备目录必须已存在，缺失或无权访问均不能自动重建。
	parts := append(strings.Split(strings.TrimPrefix(root, "/"), "/"), request.Tenant, request.Device)
	for _, part := range parts {
		next, err := openDir(fd, part)
		if err != nil {
			return Receipt{}, err
		}
		syscall.Close(fd)
		fd = next
	}
	for _, part := range []string{request.Stage, request.CaseDirectory} {
		next, err := openDir(fd, part)
		if errors.Is(err, syscall.ENOENT) {
			if err = syscall.Mkdirat(fd, part, 0770); err != nil && !errors.Is(err, syscall.EEXIST) {
				return Receipt{}, err
			}
			next, err = openDir(fd, part)
		}
		if err != nil {
			return Receipt{}, err
		}
		syscall.Close(fd)
		fd = next
	}
	var stat syscall.Stat_t
	if err = syscall.Fstat(fd, &stat); err != nil {
		return Receipt{}, err
	}
	if stat.Mode&syscall.S_IFMT != syscall.S_IFDIR {
		return Receipt{}, errors.New("not a directory")
	}
	return Receipt{Request: request, Verified: true}, nil
}
