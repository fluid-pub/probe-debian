package collect

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type FileCheck struct {
	Path        string    `json:"path"`
	Kind        string    `json:"kind"`
	Exists      bool      `json:"exists"`
	Size        int64     `json:"size"`
	Mode        string    `json:"mode"`
	OwnerUID    uint32    `json:"owner_uid"`
	OwnerGID    uint32    `json:"owner_gid"`
	ModifiedAt  time.Time `json:"modified_at,omitempty"`
	SHA256      string    `json:"sha256,omitempty"`
	EntryCount  int       `json:"entry_count,omitempty"`
	HashSkipped bool      `json:"hash_skipped"`
	Error       string    `json:"error,omitempty"`
}

func CollectFile(path string, maxHashSize int64) FileCheck {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileCheck{Path: path, Kind: "file", Exists: false}
		}
		return FileCheck{Path: path, Kind: "file", Exists: false, Error: err.Error()}
	}

	check := FileCheck{
		Path:       path,
		Kind:       "file",
		Exists:     true,
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModifiedAt: info.ModTime().UTC(),
	}
	fillOwner(&check, info)

	if info.Size() > maxHashSize {
		check.HashSkipped = true
		return check
	}

	f, err := os.Open(path)
	if err != nil {
		check.Error = err.Error()
		return check
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		check.Error = err.Error()
		return check
	}
	check.SHA256 = hex.EncodeToString(h.Sum(nil))
	return check
}

func CollectDirectory(path string, recursive bool, maxHashSize int64) FileCheck {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileCheck{Path: path, Kind: "directory", Exists: false}
		}
		return FileCheck{Path: path, Kind: "directory", Exists: false, Error: err.Error()}
	}

	check := FileCheck{
		Path:       path,
		Kind:       "directory",
		Exists:     true,
		Mode:       info.Mode().String(),
		ModifiedAt: info.ModTime().UTC(),
	}
	fillOwner(&check, info)

	count := 0
	hash := sha256.New()
	var walkErr error

	walk := func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			walkErr = err
			return nil
		}
		if filePath == path {
			return nil
		}
		if d.IsDir() && !recursive {
			return filepath.SkipDir
		}
		count++
		if d.Type().IsRegular() {
			fc := CollectFile(filePath, maxHashSize)
			io.WriteString(hash, filePath+"|"+fc.SHA256+"|"+fc.Mode+"|"+fc.ModifiedAt.Format(time.RFC3339Nano)+"\n")
		}
		return nil
	}

	_ = filepath.WalkDir(path, walk)
	check.EntryCount = count
	if walkErr != nil {
		check.Error = walkErr.Error()
	}
	check.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return check
}

func fillOwner(check *FileCheck, info os.FileInfo) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		check.OwnerUID = stat.Uid
		check.OwnerGID = stat.Gid
	}
}
