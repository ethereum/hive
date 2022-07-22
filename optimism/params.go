package optimism

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

var (
	// parameters used for signing transactions
	chainID  = big.NewInt(901)
	gasPrice = big.NewInt(30 * params.GWei)

	// would be nice to use a networkID that's different from chainID,
	// but some clients don't support the distinction properly.
	networkID = big.NewInt(901)
)
