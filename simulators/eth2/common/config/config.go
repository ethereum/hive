package config

import (
	"bytes"
	"io"
	"math/big"
)

type ForkConfig struct {
	TerminalTotalDifficulty *big.Int `json:"terminal_total_difficulty,omitempty"`
	AltairForkEpoch         *big.Int `json:"altair_fork_epoch,omitempty"`
	BellatrixForkEpoch      *big.Int `json:"bellatrix_fork_epoch,omitempty"`
	CapellaForkEpoch        *big.Int `json:"capella_fork_epoch,omitempty"`
	DenebForkEpoch          *big.Int `json:"deneb_fork_epoch,omitempty"`
}

// Choose a configuration value. `b` takes precedence
func choose(a, b *big.Int) *big.Int {
	if b != nil {
		return new(big.Int).Set(b)
	}
	if a != nil {
		return new(big.Int).Set(a)
	}
	return nil
}

// Join two configurations. `b` takes precedence
func (a *ForkConfig) Join(b *ForkConfig) *ForkConfig {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	c := ForkConfig{}
	c.AltairForkEpoch = choose(a.AltairForkEpoch, b.AltairForkEpoch)
	c.BellatrixForkEpoch = choose(a.BellatrixForkEpoch, b.BellatrixForkEpoch)
	c.CapellaForkEpoch = choose(a.CapellaForkEpoch, b.CapellaForkEpoch)
	c.DenebForkEpoch = choose(a.DenebForkEpoch, b.DenebForkEpoch)
	c.TerminalTotalDifficulty = choose(
		a.TerminalTotalDifficulty,
		b.TerminalTotalDifficulty,
	)
	return &c
}

func (c *ForkConfig) GenesisBeaconFork() string {
	if c.DenebForkEpoch != nil && c.DenebForkEpoch.Uint64() == 0 {
		return "deneb"
	} else if c.CapellaForkEpoch != nil && c.CapellaForkEpoch.Uint64() == 0 {
		return "capella"
	} else if c.BellatrixForkEpoch != nil && c.BellatrixForkEpoch.Uint64() == 0 {
		return "bellatrix"
	} else if c.AltairForkEpoch != nil && c.AltairForkEpoch.Uint64() == 0 {
		return "altair"
	} else {
		return "phase0"
	}
}

func BytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
}
