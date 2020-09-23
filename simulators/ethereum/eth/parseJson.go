package main

import (
	"encoding/json"
	"io"
)

type query struct {
	JsonRPC string `json:"jsonrpc"`
	Method string `json:"method"`
	Params []string `json:"params"`
	ID int `json:"id"`
}

type response struct {
	Result nodeInfo `json:"result"`
}

type nodeInfo struct {
	EnodeID string `json:"enode"`
}

func parse(resp io.ReadCloser) (nodeInfo, error) {
	buf := make([]byte, 100)

	bytesRead, err := resp.Read(buf)
	if err != nil {
		return nodeInfo{}, err
	}

	var response response
	if err := json.Unmarshal(buf[:bytesRead], &response); err != nil {
		return nodeInfo{}, err
	}

	return response.Result, nil
}
