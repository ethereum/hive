package mock_builder

import (
	"math/big"
	"net"
	"sync"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type PayloadAttributesModifier func(*api.PayloadAttributes, beacon.Slot) (bool, error)
type PayloadModifier func(*api.ExecutableData, beacon.Slot) (bool, error)
type ErrorProducer func(beacon.Slot) error
type PayloadWeiBidModifier func(*big.Int) (*big.Int, error)

type config struct {
	id                      int
	port                    int
	host                    string
	extraDataWatermark      string
	spec                    *beacon.Spec
	externalIP              net.IP
	beaconGenesisTime       beacon.Timestamp
	payloadWeiValueModifier PayloadWeiBidModifier

	payloadAttrModifier  PayloadAttributesModifier
	payloadModifier      PayloadModifier
	errorOnHeaderRequest ErrorProducer
	errorOnPayloadReveal ErrorProducer

	mutex sync.Mutex
}

type Option func(m *MockBuilder) error

func WithID(id int) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.id = id
		return nil
	}
}

func WithHost(host string) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.host = host
		return nil
	}
}

func WithPort(port int) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.port = port
		return nil
	}
}

func WithExtraDataWatermark(wm string) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.extraDataWatermark = wm
		return nil
	}
}

func WithExternalIP(ip net.IP) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.externalIP = ip
		return nil
	}
}

func WithSpec(spec *beacon.Spec) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.spec = spec
		return nil
	}
}

func WithBeaconGenesisTime(t beacon.Timestamp) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.beaconGenesisTime = t
		return nil
	}
}

func WithPayloadWeiValueBump(bump *big.Int) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.payloadWeiValueModifier = func(orig *big.Int) (*big.Int, error) {
			ret := new(big.Int).Set(orig)
			ret.Add(ret, bump)
			return ret, nil
		}
		return nil
	}
}

func WithPayloadWeiValueMultiplier(mult *big.Int) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.payloadWeiValueModifier = func(orig *big.Int) (*big.Int, error) {
			ret := new(big.Int).Set(orig)
			ret.Mul(ret, mult)
			return ret, nil
		}
		return nil
	}
}

func WithPayloadAttributesModifier(pam PayloadAttributesModifier) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.payloadAttrModifier = pam
		return nil
	}
}

func WithPayloadModifier(pm PayloadModifier) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.payloadModifier = pm
		return nil
	}
}

func WithErrorOnHeaderRequest(e ErrorProducer) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.errorOnHeaderRequest = e
		return nil
	}
}

func WithErrorOnPayloadReveal(e ErrorProducer) Option {
	return func(m *MockBuilder) error {
		m.cfg.mutex.Lock()
		defer m.cfg.mutex.Unlock()
		m.cfg.errorOnPayloadReveal = e
		return nil
	}
}
