package mock_builder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	el_common "github.com/ethereum/go-ethereum/common"
	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/common/builder/types/bellatrix"
	"github.com/ethereum/hive/simulators/eth2/common/builder/types/capella"
	"github.com/ethereum/hive/simulators/eth2/common/builder/types/common"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/gorilla/mux"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
	"github.com/sirupsen/logrus"
)

var (
	DOMAIN_APPLICATION_BUILDER = beacon.BLSDomainType{0x00, 0x00, 0x00, 0x01}
	EMPTY_HASH                 = el_common.Hash{}
)

type MockBuilder struct {
	// Execution and consensus clients
	el *clients.ExecutionClient
	cl *clients.BeaconClient

	// General properties
	srv      *http.Server
	sk       *blsu.SecretKey
	pk       *blsu.Pubkey
	pkBeacon beacon.BLSPubkey

	address string
	spec    *beacon.Spec
	cancel  context.CancelFunc

	// Payload/Blocks history maps
	suggestedFeeRecipients      map[beacon.BLSPubkey]el_common.Address
	suggestedFeeRecipientsMutex sync.Mutex
	builtPayloads               map[beacon.Slot]*api.ExecutableData
	builtPayloadsMutex          sync.Mutex
	modifiedPayloads            map[beacon.Slot]*api.ExecutableData
	modifiedPayloadsMutex       sync.Mutex
	signedBeaconBlock           map[tree.Root]bool
	signedBeaconBlockMutex      sync.Mutex

	// Configuration object
	cfg *config
}

const (
	DEFAULT_BUILDER_HOST = "0.0.0.0"
	DEFAULT_BUILDER_PORT = 18550
)

func NewMockBuilder(
	el *clients.ExecutionClient,
	cl *clients.BeaconClient,
	spec *beacon.Spec,
	opts ...Option,
) (*MockBuilder, error) {
	if el == nil {
		panic(fmt.Errorf("invalid EL provided: nil"))
	}
	var (
		err error
	)

	m := &MockBuilder{
		el:   el,
		cl:   cl,
		spec: spec,

		suggestedFeeRecipients: make(map[beacon.BLSPubkey]el_common.Address),
		builtPayloads:          make(map[beacon.Slot]*api.ExecutableData),
		modifiedPayloads:       make(map[beacon.Slot]*api.ExecutableData),
		signedBeaconBlock:      make(map[tree.Root]bool),

		cfg: &config{
			host: DEFAULT_BUILDER_HOST,
			port: DEFAULT_BUILDER_PORT,
			builderApiDomain: beacon.ComputeDomain(
				DOMAIN_APPLICATION_BUILDER,
				spec.GENESIS_FORK_VERSION,
				tree.Root{},
			),
		},
	}

	for _, o := range opts {
		if err = o(m); err != nil {
			return nil, err
		}
	}

	// builder key
	skByte := [32]byte{}
	sk := blsu.SecretKey{}
	rand.Read(skByte[:])
	(&sk).Deserialize(&skByte)
	m.sk = &sk
	if m.pk, err = blsu.SkToPk(m.sk); err != nil {
		panic(err)
	}
	pkBytes := m.pk.Serialize()
	copy(m.pkBeacon[:], pkBytes[:])

	router := mux.NewRouter()
	router.HandleFunc("/eth/v1/builder/validators", m.HandleValidators).
		Methods("POST")
	router.HandleFunc("/eth/v1/builder/header/{slot}/{parenthash}/{pubkey}", m.HandleGetExecutionPayloadHeader).
		Methods("GET")
	router.HandleFunc("/eth/v1/builder/blinded_blocks", m.HandleSubmitBlindedBlock).
		Methods("POST")
	router.HandleFunc("/eth/v1/builder/status", m.HandleStatus).Methods("GET")

	m.srv = &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf("%s:%d", m.cfg.host, m.cfg.port),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := m.Start(ctx); err != nil && err != context.Canceled {
			panic(err)
		}
	}()
	m.cancel = cancel

	return m, nil
}

func (m *MockBuilder) Cancel() error {
	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

// Start a proxy server.
func (m *MockBuilder) Start(ctx context.Context) error {
	m.srv.BaseContext = func(listener net.Listener) context.Context {
		return ctx
	}
	el_address := "unknown yet"

	if addr, err := m.el.UserRPCAddress(); err == nil {
		el_address = addr
	}
	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
		"address":    m.address,
		"port":       m.cfg.port,
		"el_address": el_address,
	}).Info("Builder now listening")
	go func() {
		if err := m.srv.ListenAndServe(); err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
			}).Error(err)
		}
	}()
	for {
		<-ctx.Done()
		return m.srv.Shutdown(ctx)
	}
}

func (m *MockBuilder) Address() string {
	return fmt.Sprintf(
		"http://%s@%v:%d",
		m.pkBeacon.String(),
		m.cfg.externalIP,
		m.cfg.port,
	)
}

func (m *MockBuilder) GetBuiltPayloadsCount() int {
	return len(m.builtPayloads)
}

func (m *MockBuilder) GetSignedBeaconBlockCount() int {
	return len(m.signedBeaconBlock)
}

func (m *MockBuilder) GetBuiltPayloads() map[beacon.Slot]*api.ExecutableData {
	mapCopy := make(map[beacon.Slot]*api.ExecutableData)
	for k, v := range m.builtPayloads {
		mapCopy[k] = v
	}
	return mapCopy
}

func (m *MockBuilder) GetModifiedPayloads() map[beacon.Slot]*api.ExecutableData {
	mapCopy := make(map[beacon.Slot]*api.ExecutableData)
	for k, v := range m.modifiedPayloads {
		mapCopy[k] = v
	}
	return mapCopy
}

func (m *MockBuilder) HandleValidators(
	w http.ResponseWriter,
	req *http.Request,
) {
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to read request body")
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		return
	}
	var signedValidatorRegistrations []common.SignedValidatorRegistrationV1
	if err := json.Unmarshal(requestBytes, &signedValidatorRegistrations); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request body")
		http.Error(w, "Unable to parse request body", http.StatusBadRequest)
		return
	}

	for _, vr := range signedValidatorRegistrations {
		// Verify signature
		signingRoot := beacon.ComputeSigningRoot(
			vr.Message.HashTreeRoot(tree.GetHashFn()),
			m.cfg.builderApiDomain,
		)

		pk, err := vr.Message.PubKey.Pubkey()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Unable to deserialize pubkey")
			http.Error(
				w,
				"Unable to deserialize pubkey",
				http.StatusBadRequest,
			)
			return
		}

		sig, err := vr.Signature.Signature()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Unable to deserialize signature")
			http.Error(
				w,
				"Unable to deserialize signature",
				http.StatusBadRequest,
			)
			return
		}

		if !blsu.Verify(pk, signingRoot[:], sig) {
			logrus.WithFields(logrus.Fields{
				"builder_id":    m.cfg.id,
				"pubkey":        vr.Message.PubKey,
				"fee_recipient": vr.Message.FeeRecipient,
				"timestamp":     vr.Message.Timestamp,
				"gas_limit":     vr.Message.GasLimit,
				"signature":     vr.Signature,
			}).Error("Unable to verify signature")
			http.Error(
				w,
				"Unable to verify signature",
				http.StatusBadRequest,
			)
			return
		}
		var addr el_common.Address
		copy(addr[:], vr.Message.FeeRecipient[:])
		m.suggestedFeeRecipientsMutex.Lock()
		m.suggestedFeeRecipients[vr.Message.PubKey] = addr
		m.suggestedFeeRecipientsMutex.Unlock()
	}
	logrus.WithFields(logrus.Fields{
		"builder_id":      m.cfg.id,
		"validator_count": len(signedValidatorRegistrations),
	}).Info(
		"Received validator registrations",
	)
	w.WriteHeader(http.StatusOK)

}

func (m *MockBuilder) SlotToTimestamp(slot beacon.Slot) uint64 {
	return uint64(
		m.cfg.beaconGenesisTime + beacon.Timestamp(
			slot,
		)*beacon.Timestamp(
			m.spec.SECONDS_PER_SLOT,
		),
	)
}

type PayloadHeaderRequestVarsParser map[string]string

func (vars PayloadHeaderRequestVarsParser) Slot() (slot beacon.Slot, err error) {
	if slotStr, ok := vars["slot"]; ok {
		err = (&slot).UnmarshalJSON([]byte(slotStr))
	} else {
		err = fmt.Errorf("no slot")
	}
	return slot, err
}

func (vars PayloadHeaderRequestVarsParser) PubKey() (pubkey beacon.BLSPubkey, err error) {
	if pubkeyStr, ok := vars["pubkey"]; ok {
		err = (&pubkey).UnmarshalText([]byte(pubkeyStr))
	} else {
		err = fmt.Errorf("no pubkey")
	}
	return pubkey, err
}

func (vars PayloadHeaderRequestVarsParser) ParentHash() (el_common.Hash, error) {
	if parentHashStr, ok := vars["parenthash"]; ok {
		return el_common.HexToHash(parentHashStr), nil
	}
	return el_common.Hash{}, fmt.Errorf("no parent_hash")
}

func (m *MockBuilder) HandleGetExecutionPayloadHeader(
	w http.ResponseWriter, req *http.Request,
) {
	var (
		prevRandao      el_common.Hash
		payloadModified = false
		vars            = PayloadHeaderRequestVarsParser(mux.Vars(req))
	)

	slot, err := vars.Slot()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request url")
		http.Error(
			w,
			"Unable to parse request url",
			http.StatusBadRequest,
		)
		return
	}

	parentHash, err := vars.ParentHash()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request url")
		http.Error(
			w,
			"Unable to parse request url",
			http.StatusBadRequest,
		)
		return
	}

	pubkey, err := vars.PubKey()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request url")
		http.Error(
			w,
			"Unable to parse request url",
			http.StatusBadRequest,
		)
		return
	}

	logrus.WithFields(logrus.Fields{
		"builder_id":  m.cfg.id,
		"slot":        slot,
		"parent_hash": parentHash,
		"pubkey":      pubkey,
	}).Info(
		"Received request for header",
	)
	// Request head state from the CL
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	state, err := m.cl.BeaconStateV2(ctx, eth2api.StateHead)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"slot":       slot,
			"err":        err,
		}).Error("Error getting beacon state from CL")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}
	var forkchoiceState *api.ForkchoiceStateV1
	if bytes.Equal(parentHash[:], EMPTY_HASH[:]) {
		// Edge case where the CL is requesting us to build the very first block
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		genesis, err := m.el.BlockByNumber(ctx, nil)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Error getting latest block from the EL")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}
		forkchoiceState = &api.ForkchoiceStateV1{
			HeadBlockHash: genesis.Hash(),
		}
	} else {
		// Check if we have the correct beacon state
		latestExecPayloadHeaderHash := state.LatestExecutionPayloadHeaderHash()
		if !bytes.Equal(latestExecPayloadHeaderHash[:], parentHash[:]) {
			logrus.WithFields(logrus.Fields{
				"builder_id":                  m.cfg.id,
				"latestExecPayloadHeaderHash": latestExecPayloadHeaderHash.String(),
				"parentHash":                  parentHash.String(),
				"err":                         "beacon state latest execution payload hash and parent hash requested don't match",
			}).Error("Unable to respond to header request")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}
		// Check if we know the latest forkchoice updated
		if m.el.LatestForkchoice == nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        "last fcu is unknown",
			}).Error("Unable to respond to header request")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}

		// Check if the requested parent matches the last fcu
		if !bytes.Equal(m.el.LatestForkchoice.HeadBlockHash[:], parentHash[:]) {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        "last fcu head and requested parent don't match",
			}).Error("Unable to respond to header request")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}

		forkchoiceState = m.el.LatestForkchoice
	}

	// Build payload attributes

	// PrevRandao
	prevRandaoMixes := state.RandaoMixes()
	prevRandaoRoot := prevRandaoMixes[m.spec.SlotToEpoch(slot-1)]
	copy(prevRandao[:], prevRandaoRoot[:])

	// Timestamp
	timestamp := m.SlotToTimestamp(slot)

	// Suggested Fee Recipient
	suggestedFeeRecipient := m.suggestedFeeRecipients[pubkey]

	// Withdrawals
	var withdrawals types.Withdrawals
	if m.spec.SlotToEpoch(slot) >= m.spec.CAPELLA_FORK_EPOCH {
		wSsz, err := state.NextWithdrawals(slot)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Unable to obtain correct list of withdrawals")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}
		withdrawals = make(types.Withdrawals, len(wSsz))
		for i, w := range wSsz {
			newWithdrawal := types.Withdrawal{}
			copy(newWithdrawal.Address[:], w.Address[:])
			newWithdrawal.Amount = uint64(w.Amount)
			newWithdrawal.Index = uint64(w.Index)
			newWithdrawal.Validator = uint64(w.ValidatorIndex)
			withdrawals[i] = &newWithdrawal
		}
	}

	pAttr := api.PayloadAttributes{
		Timestamp:             timestamp,
		Random:                prevRandao,
		SuggestedFeeRecipient: suggestedFeeRecipient,
		Withdrawals:           withdrawals,
	}

	if m.cfg.payloadAttrModifier != nil {
		if mod, err := m.cfg.payloadAttrModifier(&pAttr, slot); err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Unable to modify payload attributes using modifier")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		} else if mod {
			payloadModified = true
		}
	}

	logrus.WithFields(logrus.Fields{
		"builder_id":            m.cfg.id,
		"Timestamp":             timestamp,
		"PrevRandao":            prevRandao,
		"SuggestedFeeRecipient": suggestedFeeRecipient,
		"Withdrawals":           withdrawals,
	}).Info("Built payload attributes for header")

	// Request a payload from the execution client
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := m.el.EngineForkchoiceUpdated(
		ctx,
		forkchoiceState,
		&pAttr,
		2,
	)
	if err != nil || r.PayloadID == nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
			"payloadID":  r.PayloadID,
		}).Error("Error on ForkchoiceUpdated to EL")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	// Wait for EL to produce payload
	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
		"payloadID":  r.PayloadID.String(),
	}).Info("Waiting for payload from EL")

	time.Sleep(200 * time.Millisecond)

	// Request payload from the EL
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p, bValue, err := m.el.EngineGetPayload(ctx, r.PayloadID, 2)
	if err != nil || p == nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
			"payload":    p,
		}).Error("Error on GetPayload to EL")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	// Watermark payload
	if err := ModifyExtraData(p, []byte("builder payload")); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Error modifying payload")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	// Modify the payload if necessary
	if m.cfg.payloadModifier != nil {
		if mod, err := m.cfg.payloadModifier(p, slot); err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"err":        err,
			}).Error("Error modifying payload")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		} else if mod {
			payloadModified = true
		}
	}

	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
		"payload":    p.BlockHash.String(),
	}).Info("Built payload from EL")

	// We are ready to respond to the CL
	var (
		builderBid common.BuilderBid
		version    string
	)

	if m.spec.SlotToEpoch(slot) >= m.spec.CAPELLA_FORK_EPOCH {
		builderBid = &capella.BuilderBid{}
		version = "capella"
	} else if m.spec.SlotToEpoch(slot) >= m.spec.BELLATRIX_FORK_EPOCH {
		builderBid = &bellatrix.BuilderBid{}
		version = "bellatrix"
	} else {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        fmt.Errorf("payload requested from improper fork"),
		}).Error("Invalid slot requested")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusBadRequest,
		)
		return
	}

	if err := builderBid.FromExecutableData(m.spec, p); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Error building bid from execution data")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	if m.cfg.payloadWeiValueBump != nil {
		// If requested, fake a higher gwei so the CL always takes the bid
		bValue = bValue.Add(bValue, m.cfg.payloadWeiValueBump)
	}
	builderBid.SetValue(bValue)
	builderBid.SetPubKey(m.pkBeacon)

	signedBid, err := builderBid.Sign(m.cfg.builderApiDomain, m.sk, m.pk)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Error signing bid from execution data")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	// Check if we are supposed to simulate an error
	if m.cfg.errorOnHeaderRequest != nil {
		if err := m.cfg.errorOnHeaderRequest(slot); err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"slot":       slot,
				"err":        err,
			}).Error("Simulated error")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}
	}

	versionedSignedBid := signedBid.Versioned(version)
	if err := serveJSON(w, versionedSignedBid); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Error versioning bid from execution data")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}

	// Finally add the execution payload to the cache
	m.builtPayloadsMutex.Lock()
	m.builtPayloads[slot] = p
	m.builtPayloadsMutex.Unlock()
	if payloadModified {
		m.modifiedPayloadsMutex.Lock()
		m.modifiedPayloads[slot] = p
		m.modifiedPayloadsMutex.Unlock()
	}
}

type SlotEnvelope struct {
	Slot beacon.Slot `json:"slot" yaml:"slot"`
}

type MessageSlotEnvelope struct {
	SlotEnvelope SlotEnvelope `json:"message" yaml:"message"`
}

func (m *MockBuilder) HandleSubmitBlindedBlock(
	w http.ResponseWriter, req *http.Request,
) {
	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
	}).Info(
		"Received submission for blinded blocks",
	)
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to read request body")
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		return
	}

	// First try to find out the slot to get the version of the block
	var messageSlotEnvelope MessageSlotEnvelope
	if err := json.Unmarshal(requestBytes, &messageSlotEnvelope); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request body")
		http.Error(w, "Unable to parse request body", http.StatusBadRequest)
		return
	}

	var (
		signedBeaconBlock    common.SignedBlindedBeacon
		executionPayloadResp common.ExecutionPayloadResponse
	)
	if m.spec.SlotToEpoch(
		messageSlotEnvelope.SlotEnvelope.Slot,
	) >= m.spec.CAPELLA_FORK_EPOCH {
		signedBeaconBlock = &capella.SignedBeaconBlock{}
		executionPayloadResp.Version = "capella"
		executionPayloadResp.Data = &capella.ExecutionPayload{}
	} else if m.spec.SlotToEpoch(messageSlotEnvelope.SlotEnvelope.Slot) >= m.spec.BELLATRIX_FORK_EPOCH {
		signedBeaconBlock = &bellatrix.SignedBeaconBlock{}
		executionPayloadResp.Version = "bellatrix"
		executionPayloadResp.Data = &bellatrix.ExecutionPayload{}
	} else {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        fmt.Errorf("received signed beacon blinded block of unknown fork"),
		}).Error("Invalid slot requested")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusBadRequest,
		)
		return
	}
	// Unmarshall the full signed beacon block
	if err := json.Unmarshal(requestBytes, &signedBeaconBlock); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Unable to parse request body")
		http.Error(w, "Unable to parse request body", http.StatusBadRequest)
		return
	}

	// Look up the payload in the history of payloads
	p, ok := m.builtPayloads[messageSlotEnvelope.SlotEnvelope.Slot]
	if !ok {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"slot":       messageSlotEnvelope.SlotEnvelope.Slot,
		}).Error("Could not find payload in history")
		http.Error(w, "Unable to get payload", http.StatusInternalServerError)
		return
	}

	// Prepare response
	executionPayloadResp.Data.FromExecutableData(p)

	// Embed the execution payload in the block to obtain correct root
	signedBeaconBlock.SetExecutionPayload(
		executionPayloadResp.Data,
	)

	// Record the signed beacon block
	signedBeaconBlockRoot := signedBeaconBlock.Root(m.spec)
	m.signedBeaconBlockMutex.Lock()
	m.signedBeaconBlock[signedBeaconBlockRoot] = true
	m.signedBeaconBlockMutex.Unlock()

	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
		"root":       signedBeaconBlock.Root(m.spec).String(),
		"stateRoot":  signedBeaconBlock.StateRoot().String(),
		"slot":       signedBeaconBlock.Slot().String(),
	}).Info("Received signed beacon block")

	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
		"payload":    p.BlockHash.String(),
	}).Info("Built payload sent to CL")

	// Check if we are supposed to simulate an error
	if m.cfg.errorOnPayloadReveal != nil {
		if err := m.cfg.errorOnPayloadReveal(messageSlotEnvelope.SlotEnvelope.Slot); err != nil {
			logrus.WithFields(logrus.Fields{
				"builder_id": m.cfg.id,
				"slot":       messageSlotEnvelope.SlotEnvelope.Slot,
				"err":        err,
			}).Error("Simulated error")
			http.Error(
				w,
				"Unable to respond to header request",
				http.StatusInternalServerError,
			)
			return
		}
	}

	if err := serveJSON(w, executionPayloadResp); err != nil {
		logrus.WithFields(logrus.Fields{
			"builder_id": m.cfg.id,
			"err":        err,
		}).Error("Error preparing response from payload")
		http.Error(
			w,
			"Unable to respond to header request",
			http.StatusInternalServerError,
		)
		return
	}
}

func (m *MockBuilder) HandleStatus(
	w http.ResponseWriter, req *http.Request,
) {
	logrus.WithFields(logrus.Fields{
		"builder_id": m.cfg.id,
	}).Info(
		"Received request for status",
	)
	w.WriteHeader(http.StatusOK)
}

func serveJSON(w http.ResponseWriter, value interface{}) error {
	resp, err := json.Marshal(value)
	if err != nil {
		return err
	}
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
	return nil
}

func ModifyExtraData(p *api.ExecutableData, newExtraData []byte) error {
	if p == nil {
		return fmt.Errorf("nil payload")
	}
	if b, err := api.ExecutableDataToBlock(*p); err != nil {
		return err
	} else {
		h := b.Header()
		h.Extra = newExtraData
		p.ExtraData = newExtraData
		p.BlockHash = h.Hash()
	}
	return nil
}
