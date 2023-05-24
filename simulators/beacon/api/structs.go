package main

type Status struct {
	ForkDigest     []byte `json:"fork_digest" ssz-size:"4"`
	FinalizedRoot  []byte `json:"finalized_root" ssz-size:"32"`
	FinalizedEpoch uint64 `json:"finalized_epoch"`
	HeadRoot       []byte `json:"head_root" ssz-size:"32"`
	HeadSlot       uint64 `json:"head_slot"`
}
