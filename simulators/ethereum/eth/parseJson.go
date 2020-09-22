package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type response struct {
	Result nodeInfo `json:"result"`
}

type nodeInfo struct {
	EnodeID string `json:"enode"`
}

func main() {
	absPath, _ := filepath.Abs("./enode.json")
	file, err := os.Open(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not open file, %v", err)
		os.Exit(1)
	}

	info, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get file stats, %v", err)
		os.Exit(1)
	}

	buf := make([]byte, info.Size())
	_, err = file.Read(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not read file, %v", err)
		os.Exit(1)
	}

	var res response
	err = json.Unmarshal(buf, &res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not unmarshal response, %v", err)
		os.Exit(1)
	}

	enode := res.Result.EnodeID
	fmt.Fprint(os.Stdout, enode[:len(enode)-11])
}
