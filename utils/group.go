package utils

import (
	"io/fs"
	"path/filepath"
)

func GetTestCgroupPath() ([]string, error) {
	dirs := []string{}

	path := "/sys/fs/cgroup/kube"
	err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return dirs, nil
}
