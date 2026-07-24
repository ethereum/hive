package main

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// TestCallTreeCalleeAddresses guards the coupling between the callee address
// constants in genesis.go and the addresses compiled into calltree.bin from
// contracts/calltree.eas.
func TestCallTreeCalleeAddresses(t *testing.T) {
	callees := map[string]string{
		"callme":     calltreeCallmeAddr,
		"callenv":    calltreeCallenvAddr,
		"callrevert": calltreeCallrevertAddr,
		"emit":       emitAddr,
	}
	for name, addr := range callees {
		if !bytes.Contains(calltreeCode, common.HexToAddress(addr).Bytes()) {
			t.Errorf("calltree.bin does not contain the %s address %s; regenerate with compile.sh after changing addresses", name, addr)
		}
	}
}
