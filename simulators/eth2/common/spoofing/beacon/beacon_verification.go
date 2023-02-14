package beacon

import (
	"fmt"

	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/simulators/eth2/common/spoofing/proxy"
	spoof "github.com/rauljordan/engine-proxy/proxy"
	"golang.org/x/exp/slices"
)

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineForkchoiceUpdatedV2 = "engine_forkchoiceUpdatedV2"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EngineNewPayloadV2        = "engine_newPayloadV2"
)

var EngineNewPayload = []string{
	EngineNewPayloadV1,
	EngineNewPayloadV2,
}

var EngineForkchoiceUpdated = []string{
	EngineForkchoiceUpdatedV1,
	EngineForkchoiceUpdatedV2,
}

type FallibleLogger interface {
	Fail()
	Logf(format string, values ...interface{})
}

func GetTimestampFromNewPayload(
	req []byte,
) (*uint64, error) {
	var payload api.ExecutableData
	if err := proxy.UnmarshalFromJsonRPCRequest(req, &payload); err != nil {
		return nil, err
	}
	return &payload.Timestamp, nil
}

func GetTimestampFromFcU(
	req []byte,
) (*uint64, error) {
	var (
		fcS api.ForkchoiceStateV1
		pA  *api.PayloadAttributes
	)
	if err := proxy.UnmarshalFromJsonRPCRequest(req, &fcS, &pA); err != nil {
		return nil, err
	}
	if pA == nil {
		return nil, nil
	}
	return &pA.Timestamp, nil
}

type EngineEndpointMaxTimestampVerify struct {
	Endpoint          string
	ExpiringTimestamp uint64
	GetTimestampFn    func([]byte) (*uint64, error)
	FallibleLogger    FallibleLogger
}

func (v *EngineEndpointMaxTimestampVerify) Verify(
	req []byte,
) *spoof.Spoof {
	if v.GetTimestampFn == nil {
		panic(fmt.Errorf("timestamp parse function not specified"))
	}
	timestamp, err := v.GetTimestampFn(req)
	if err != nil {
		panic(err)
	}
	if timestamp != nil && *timestamp >= v.ExpiringTimestamp {
		if v.FallibleLogger == nil {
			panic(fmt.Errorf("test is nil"))
		}
		v.FallibleLogger.Logf(
			"FAIL: received directive using expired endpoint %s: timestamp %d >= %d",
			v.Endpoint,
			*timestamp,
			v.ExpiringTimestamp,
		)
		v.FallibleLogger.Fail()
	}
	return nil
}

func (v *EngineEndpointMaxTimestampVerify) AddToProxy(p *proxy.Proxy) error {
	if p == nil {
		return fmt.Errorf("attempted to add to nil proxy")
	}
	if v.Endpoint == "" {
		return fmt.Errorf("attempted to add to proxy with empty endpoint")
	}
	p.AddRequestCallback(v.Endpoint, v.Verify)
	return nil
}

func NewEngineMaxTimestampVerifier(
	t FallibleLogger,
	endpoint string,
	expiringTimestamp uint64,
) *EngineEndpointMaxTimestampVerify {
	var getTimestampFn func([]byte) (*uint64, error)
	if slices.Contains(EngineNewPayload, endpoint) {
		getTimestampFn = GetTimestampFromNewPayload
	} else if slices.Contains(EngineForkchoiceUpdated, endpoint) {
		getTimestampFn = GetTimestampFromFcU
	} else {
		panic(fmt.Errorf("invalid endpoint for verification"))
	}

	return &EngineEndpointMaxTimestampVerify{
		Endpoint:          endpoint,
		ExpiringTimestamp: expiringTimestamp,
		FallibleLogger:    t,
		GetTimestampFn:    getTimestampFn,
	}
}
