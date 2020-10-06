package main

import (
	"encoding/json"
	"io/ioutil"
	"sync"
)

type HiveErrorReport struct {
	mu     sync.Mutex
	Errors []ContainerError `json:"errors"`
}

type ContainerError struct {
	Name    string `json:"name"`
	Details string `json:"details"`
}

func NewHiveErrorReport() *HiveErrorReport {
	return &HiveErrorReport{}
}

func (h *HiveErrorReport) AddErrorReport(containerError ContainerError) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Errors = append(h.Errors, containerError)
}

func (h *HiveErrorReport) WriteReport(outputPath string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := json.Marshal(h.Errors)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(outputPath, data, 0644)
}
