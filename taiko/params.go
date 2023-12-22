package taiko

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

var (
	gasPrice = big.NewInt(30 * params.GWei)
)
