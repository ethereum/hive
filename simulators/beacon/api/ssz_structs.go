package main

//go:generate go run github.com/prysmaticlabs/fastssz/sszgen --path $GOFILE

type Status struct {
	ForkDigest     []byte `json:"fork_digest" ssz-size:"4"`
	FinalizedRoot  []byte `json:"finalized_root" ssz-size:"32"`
	FinalizedEpoch uint64 `json:"finalized_epoch"`
	HeadRoot       []byte `json:"head_root" ssz-size:"32"`
	HeadSlot       uint64 `json:"head_slot"`
}

type MetaData struct {
	SeqNumber uint64 `json:"seq_number"`
	AttNets   []byte `json:"attnets" ssz:"bitlist" ssz-max:"64"`
}

type BlocksByRangeRequest struct {
	StartSlot uint64 `json:"start_slot"`
	Count     uint64 `json:"count"`
	Step      uint64 `json:"step"`
}
