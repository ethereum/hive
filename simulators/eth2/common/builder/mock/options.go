package mock_builder

import (
	"math/big"
	"net"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type PayloadAttributesModifier func(*api.PayloadAttributes, beacon.Slot) (bool, error)
type PayloadModifier func(*api.ExecutableData, beacon.Slot) (bool, error)
type ErrorProducer func(beacon.Slot) error

type config struct {
	id                  int
	port                int
	host                string
	externalIP          net.IP
	builderApiDomain    beacon.BLSDomain
	beaconGenesisTime   beacon.Timestamp
	payloadWeiValueBump *big.Int

	payloadAttrModifier  PayloadAttributesModifier
	payloadModifier      PayloadModifier
	errorOnHeaderRequest ErrorProducer
	errorOnPayloadReveal ErrorProducer
}

type Option func(m *MockBuilder) error

func WithID(id int) Option {
	return func(m *MockBuilder) error {
		m.cfg.id = id
		return nil
	}
}

func WithHost(host string) Option {
	return func(m *MockBuilder) error {
		m.cfg.host = host
		return nil
	}
}

func WithPort(port int) Option {
	return func(m *MockBuilder) error {
		m.cfg.port = port
		return nil
	}
}

func WithExternalIP(ip net.IP) Option {
	return func(m *MockBuilder) error {
		m.cfg.externalIP = ip
		return nil
	}
}

func WithExecutionClient() Option {
	return func(m *MockBuilder) error {
		return nil
	}
}

func WithBuilderApiDomain(domain beacon.BLSDomain) Option {
	return func(m *MockBuilder) error {
		m.cfg.builderApiDomain = domain
		return nil
	}
}

func WithBeaconGenesisTime(t beacon.Timestamp) Option {
	return func(m *MockBuilder) error {
		m.cfg.beaconGenesisTime = t
		return nil
	}
}

func WithPayloadWeiValueBump(wei *big.Int) Option {
	return func(m *MockBuilder) error {
		m.cfg.payloadWeiValueBump = wei
		return nil
	}
}

func WithPayloadAttributesModifier(pam PayloadAttributesModifier) Option {
	return func(m *MockBuilder) error {
		m.cfg.payloadAttrModifier = pam
		return nil
	}
}

func WithPayloadModifier(pm PayloadModifier) Option {
	return func(m *MockBuilder) error {
		m.cfg.payloadModifier = pm
		return nil
	}
}

func WithErrorOnHeaderRequest(e ErrorProducer) Option {
	return func(m *MockBuilder) error {
		m.cfg.errorOnHeaderRequest = e
		return nil
	}
}

func WithErrorOnPayloadReveal(e ErrorProducer) Option {
	return func(m *MockBuilder) error {
		m.cfg.errorOnPayloadReveal = e
		return nil
	}
}
