package recorder

import (
	"os"
	"path/filepath"
	"sort"
)

type recInfo struct {
	id    string
	path  string
	start int64
	size  int64
}

func enforceQuota(root string, maxMB int) {
	if maxMB <= 0 {
		return
	}
	maxBytes := int64(maxMB) * 1024 * 1024
	infos, total := scanRecordings(root)
	if total <= maxBytes {
		return
	}
	// oldest first
	sort.Slice(infos, func(i, j int) bool { return infos[i].start < infos[j].start })
	for _, info := range infos {
		if total <= maxBytes {
			break
		}
		_ = os.RemoveAll(info.path)
		total -= info.size
	}
}

func scanRecordings(root string) ([]recInfo, int64) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0
	}
	infos := make([]recInfo, 0, len(entries))
	var total int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		path := filepath.Join(root, id)
		size := dirSize(path)
		start := int64(0)
		if meta, err := ReadMeta(filepath.Join(path, "meta.json")); err == nil {
			start = meta.Start.UnixMilli()
		}
		infos = append(infos, recInfo{id: id, path: path, start: start, size: size})
		total += size
	}
	return infos, total
}

func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
