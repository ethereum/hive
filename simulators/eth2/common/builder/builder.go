package builder

import (
	api "github.com/ethereum/go-ethereum/core/beacon"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type Builder interface {
	Address() string
	Cancel() error
	GetBuiltPayloadsCount() int
	GetSignedBeaconBlockCount() int
	GetModifiedPayloads() map[beacon.Slot]*api.ExecutableData
	GetBuiltPayloads() map[beacon.Slot]*api.ExecutableData
}
