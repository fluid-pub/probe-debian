package shipper

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

type Spool struct {
	path       string
	maxEntries int
}

func NewSpool(path string, maxEntries int) *Spool {
	return &Spool{path: path, maxEntries: maxEntries}
}

func (s *Spool) Append(item map[string]interface{}) error {
	items, _ := s.LoadAll()
	items = append(items, item)
	if len(items) > s.maxEntries {
		items = items[len(items)-s.maxEntries:]
	}
	return s.overwrite(items)
}

func (s *Spool) LoadAll() ([]map[string]interface{}, error) {
	f, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	items := make([]map[string]interface{}, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var row map[string]interface{}
		if err := json.Unmarshal(line, &row); err == nil {
			items = append(items, row)
		}
	}
	return items, scanner.Err()
}

func (s *Spool) RemoveFirst(n int) error {
	items, err := s.LoadAll()
	if err != nil {
		return err
	}
	if n >= len(items) {
		return s.overwrite(nil)
	}
	return s.overwrite(items[n:])
}

func (s *Spool) overwrite(items []map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}
