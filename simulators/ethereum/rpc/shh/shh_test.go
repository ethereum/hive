package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p"
	whisper "github.com/ethereum/go-ethereum/whisper/whisperv5"
)

var (
	ips []string // contains list of ip addresses of running nodes
)

func TestMain(m *testing.M) {
	flag.Parse()
	ips = flag.Args() // args contains list of node ip addresses

	rand.Seed(time.Now().Unix())

	os.Exit(m.Run())
}

func TestShh(t *testing.T) {
	// shh client test suite
	t.Run("websocket", func(t *testing.T) {
		clients := createWebsocketClients(ips)

		t.Run("MaxMessageSize", runTest(maxMessageSizeTest, clients))
		t.Run("MinimumPoW", runTest(minPoWTest, clients))
		t.Run("PublicKeySendAndReceive", runTest(publicKeyTest, clients))
		t.Run("PublicKeySendAndReceiveWithTopics", runTest(publicKeyWithTopicsAndSubscriptionsTest, clients))
		t.Run("PublicKeyWithTopicsAndPolling", runTest(publicKeyWithTopicsAndPollingTest, clients))
		t.Run("SymmetricKeySendAndReceiveWithTopics", runTest(symmetricKeyWithTopicsAndSubscriptionsTest, clients))
		t.Run("SymmetricKeyWithTopicsAndPolling", runTest(symmetricKeyWithTopicsAndPollingTest, clients))
		t.Run("DirectP2PMessaging", runTest(directP2PMessagingTest, clients))
		t.Run("TrustedPeer", runTest(trustedPeerTest, clients))
	})

	t.Run("http", func(t *testing.T) {
		clients := createHTTPClients(ips)

		t.Run("MaxMessageSize", runTest(maxMessageSizeTest, clients))
		t.Run("MinimumPoW", runTest(minPoWTest, clients))
		t.Run("PublicKeySendAndReceive", runTest(publicKeyWithPollingTest, clients))
		t.Run("PublicKeyWithTopicsAndPolling", runTest(publicKeyWithTopicsAndPollingTest, clients))
		t.Run("SymmetricKeyWithTopicsAndPolling", runTest(symmetricKeyWithTopicsAndPollingTest, clients))
		t.Run("DirectP2PMessagingWithPolling", runTest(directP2PMessagingWithPollingTest, clients))
		t.Run("TrustedPeer", runTest(trustedPeerTest, clients))
	})
}

// minPoWTest ensures that messages with a too small PoW are rejected.
func minPoWTest(t *testing.T, clients []*TestClient) {
	var (
		client = clients[0]
		newPoW = 0.5
	)

	_, pubKey, _ := generateAsymKey(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	if err := client.SetMinimumPoW(ctx, newPoW); err != nil {
		t.Fatalf("could not set new minimum PoW: %v", err)
	}
	cancel()

	// restore default minimum accepted PoW after test finished.
	defer func() {
		ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
		if err := client.SetMinimumPoW(ctx, whisper.DefaultMinimumPoW); err != nil {
			t.Fatalf("could not restore default minimum PoW: %v", err)
		}
		cancel()
	}()

	// send message with too low PoW and ensure it is rejected
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	msg := whisper.NewMessage{
		PublicKey: pubKey,
		Payload:   []byte("abcd"),
		PowTarget: newPoW + 0.01,
		PowTime:   10,
	}
	if err := client.Post(ctx, msg); err != nil {
		t.Fatalf("could not post message: %v", err)
	}
	cancel()

	// send message with large enough PoW and ensure that it is accepted
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	msg = whisper.NewMessage{
		PublicKey: pubKey,
		Payload:   []byte("abcd"),
		PowTarget: newPoW - 0.01,
		PowTime:   10,
	}

	if err := client.Post(ctx, msg); err == nil {
		t.Fatalf("post message should have failed due to too low PoW")
	}
}

// maxMessageSizeTest ensures that messages larger than the set max message
// size are rejected.
func maxMessageSizeTest(t *testing.T, clients []*TestClient) {
	var (
		client            = clients[0]
		newMaxMessageSize = 1024

		smallPayload  = bytes.Repeat([]byte{1}, newMaxMessageSize/2)
		tooBigPayload = bytes.Repeat([]byte{1}, newMaxMessageSize)
	)

	_, pubKey, _ := generateAsymKey(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	if err := client.SetMaxMessageSize(ctx, uint32(newMaxMessageSize)); err != nil {
		t.Fatalf("could not set max message size: %v", err)
	}
	cancel()

	// restore default max message after test finished.
	defer func() {
		ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
		if err := client.SetMaxMessageSize(ctx, whisper.DefaultMaxMessageSize); err != nil {
			t.Fatalf("could not restore default max message size: %v", err)
		}
		cancel()
	}()

	// send message with size < newMaxMessageSize
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	msg := whisper.NewMessage{
		PublicKey: pubKey,
		Payload:   smallPayload,
		PowTarget: 0.25,
		PowTime:   5,
	}

	if err := client.Post(ctx, msg); err != nil {
		t.Fatalf("could not post message: %v", err)
	}

	// send message with size > newMaxMessageSize
	msg = whisper.NewMessage{
		PublicKey: pubKey,
		Payload:   tooBigPayload,
		PowTarget: 0.25,
		PowTime:   5,
	}

	if err := client.Post(ctx, msg); err == nil {
		t.Fatal("message post should have failed due to too large payload")
	}
}

// generateAsymKey is a helper function that generates a key pair in the node
// and returns the identifier, public and private key. In case on an error this
// function calls t.Fatalf.
func generateAsymKey(t *testing.T, client *TestClient) (string, []byte, []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	keyID, err := client.NewKeyPair(ctx)
	if err != nil {
		t.Fatalf("could not generate new key pair: %v", err)
	}
	pubKey, err := client.PublicKey(ctx, keyID)
	if err != nil {
		t.Fatalf("could not retrieve public key: %v", err)
	}
	privKey, err := client.PrivateKey(ctx, keyID)
	if err != nil {
		t.Fatalf("could not retrieve private key")
	}
	return keyID, pubKey, privKey
}

// addSymmetricKey is a helper function that imports the given symmetric key
// into the client and returns the key identifier. In case of an error it
// calls t.Fatalf.
func addSymmetricKey(t *testing.T, client *TestClient, passwd []byte) string {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	key, err := client.AddSymmetricKey(ctx, passwd)
	if err != nil {
		t.Fatalf("could not import symmetric key: %v", err)
	}
	return key
}

// createMessageSubscription is a helper function that creates a subscription
// that fires when messages that satisfy the given criteria are received.
func createMessageSubscription(t *testing.T, client *TestClient, crit whisper.Criteria) (ethereum.Subscription, chan *whisper.Message) {
	messages := make(chan *whisper.Message)
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	sub, err := client.SubscribeMessages(ctx, crit, messages)
	if err != nil {
		t.Fatalf("could not create subscription: %v", err)
	}
	return sub, messages
}

func createMessageFilter(t *testing.T, client *TestClient, crit whisper.Criteria) string {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	id, err := client.NewMessageFilter(ctx, crit)
	if err != nil {
		t.Fatalf("Could not create message filter: %v", err)
	}
	return id
}

func symmetricKeyWithTopicsAndPollingTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	var (
		sharedPasswd = crypto.Keccak256Hash([]byte("symmetricKeyWithTopicsAndPollingTestPassword")).Bytes()
		topics       = randTopics(3)
		clientA      = clients[0]
		clientB      = clients[1]
		clientC      = clients[2]
	)

	// create and share private keys between nodes
	keyA := addSymmetricKey(t, clientA, sharedPasswd)
	keyB := addSymmetricKey(t, clientB, sharedPasswd)
	keyC := addSymmetricKey(t, clientC, sharedPasswd)

	// create message filters
	filterA := createMessageFilter(t, clientA, whisper.Criteria{SymKeyID: keyA, Topics: topics})
	filterB := createMessageFilter(t, clientB, whisper.Criteria{SymKeyID: keyB, Topics: topics})
	filterC := createMessageFilter(t, clientC, whisper.Criteria{SymKeyID: keyC, Topics: topics})

	// cleanup filters
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*rpcTimeout)
		if err := clientA.DeleteMessageFilter(ctx, filterA); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		if err := clientB.DeleteMessageFilter(ctx, filterB); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		if err := clientC.DeleteMessageFilter(ctx, filterC); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		cancel()
	}()

	// send a bunch of messages to each other
	for i := 1; i < 3; i++ {
		// prepare 2 messages, each one for the other node than the node
		// that is randomly selected to post to.
		msg := whisper.NewMessage{
			Topic:     topics[i%len(topics)],
			PowTarget: whisper.DefaultMinimumPoW,
			PowTime:   5,
			TTL:       60,
		}

		// randomly select a client for posting the messages and prepare messages to post
		var source *TestClient
		switch rand.Int31n(3) {
		case 0:
			source = clientA
			msg.Payload, msg.SymKeyID = []byte(fmt.Sprintf("from A: %d", i)), keyA
			//targets[0], targets[1], filterIDs[0], filterIDs[1] = clientB, clientC, filterB, filterC
		case 1:
			source = clientB
			msg.Payload, msg.SymKeyID = []byte(fmt.Sprintf("from B: %d", i)), keyB
			//targets[0], targets[1], filterIDs[0], filterIDs[1] = clientC, clientA, filterC, filterA
		case 2:
			source = clientC
			msg.Payload, msg.SymKeyID = []byte(fmt.Sprintf("from C: %d", i)), keyC
			//targets[0], targets[1], filterIDs[0], filterIDs[1] = clientA, clientB, filterA, filterB
		}

		// post message
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		if err := source.Post(ctx, msg); err != nil {
			t.Fatalf("could not post message: %v", err)
		}
		cancel()

		// poll nodes for new messages
		var wait sync.WaitGroup
		poller := func(t *testing.T, client *TestClient, filter string, expected whisper.NewMessage) {
			defer wait.Done()

			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
				received, err := client.FilterMessages(ctx, filter)
				cancel()

				if err != nil {
					t.Fatalf("could not poll for new messages: %v", err)
				}

				if len(received) > 0 {
					msg := received[0]
					if expected.Topic != msg.Topic {
						t.Errorf("unexpected topic, want %x, got %x", expected.Topic, msg.Topic)
					}
					if bytes.Compare(expected.Payload, msg.Payload) != 0 {
						t.Errorf("unexpected payload, want %s, got %s", expected.Payload, msg.Payload)
					}
					return
				}

				<-ticker.C
			}

			t.Errorf("didn't receive message within reasonable time")
		}

		wait.Add(3)
		go poller(t, clientA, filterA, msg)
		go poller(t, clientB, filterB, msg)
		go poller(t, clientC, filterC, msg)
		wait.Wait()
	}
}

// symmetricKeyWithTopicsAndSubscriptionsTest simulates 3 whisper clients sending messages
// to each other using symmetric key encryption. This tests ensures no messages are lost
// and the payload of received messages is as expected.
func symmetricKeyWithTopicsAndSubscriptionsTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	var (
		sharedPasswd = crypto.Keccak256Hash([]byte("symmetricKeyWithTopicsAndSubscriptionsTestPassword")).Bytes()
		topics       = randTopics(1)
		clientA      = clients[0]
		clientB      = clients[1]
		clientC      = clients[2]
	)

	// create and share private keys between nodes
	keyA := addSymmetricKey(t, clientA, sharedPasswd)
	keyB := addSymmetricKey(t, clientB, sharedPasswd)
	keyC := addSymmetricKey(t, clientC, sharedPasswd)

	// create subscription that will emit a message when the shared password was
	// used to encrypt the message.
	subA, inA := createMessageSubscription(t, clientA, whisper.Criteria{SymKeyID: keyA, Topics: topics})
	subB, inB := createMessageSubscription(t, clientB, whisper.Criteria{SymKeyID: keyB, Topics: topics})
	subC, inC := createMessageSubscription(t, clientC, whisper.Criteria{SymKeyID: keyC, Topics: topics})

	// post and wait for messages
	for i := 0; i < 50; i++ {
		msg := whisper.NewMessage{
			Topic:     topics[0],
			PowTarget: whisper.DefaultMinimumPoW,
			PowTime:   5,
		}

		// randomly select a client for posting the message
		var client *TestClient
		switch rand.Int31n(3) {
		case 0:
			client = clientA
			msg.Payload = []byte(fmt.Sprintf("from A: %d", i))
			msg.SymKeyID = keyA
		case 1:
			client = clientB
			msg.Payload = []byte(fmt.Sprintf("from B: %d", i))
			msg.SymKeyID = keyB
		case 2:
			client = clientC
			msg.Payload = []byte(fmt.Sprintf("from C: %d", i))
			msg.SymKeyID = keyC
		}

		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		if err := client.Post(ctx, msg); err != nil {
			t.Fatalf("could not post message: %v", err)
		}
		cancel()

		var (
			msgA *whisper.Message
			msgB *whisper.Message
			msgC *whisper.Message
		)

		timeout := time.NewTimer(10 * time.Second)
		for msgA == nil || msgB == nil || msgC == nil {
			select {
			case msgA = <-inA:
				if bytes.Compare(msg.Payload, msg.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msg.Payload, msgA.Payload)
				}
			case msgB = <-inB:
				if bytes.Compare(msg.Payload, msgB.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msg.Payload, msgB.Payload)
				}
			case msgC = <-inC:
				if bytes.Compare(msg.Payload, msgC.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msg.Payload, msgC.Payload)
				}
			case err := <-subA.Err():
				t.Fatalf("subscription returned err: %v", err)
			case err := <-subB.Err():
				t.Fatalf("subscription returned err: %v", err)
			case err := <-subC.Err():
				t.Fatalf("subscription returned err: %v", err)
			case <-timeout.C:
				t.Fatal("didn't receive message within deadline")
			}
		}
	}
}

func publicKeyWithPollingTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	type clientContext struct {
		client   *TestClient
		pubKeyID string
		pubKey   []byte
		mu       sync.Mutex
		send     map[string]bool
		received map[string]bool

		filter    string
		nMessages int
		sub       ethereum.Subscription
	}

	var (
		rootCtx        = context.Background()
		clientContexts = make([]*clientContext, len(clients))
		expectedToReceive int
	)

	// generate client context
	for i, client := range clients[:3] {
		ctx, cancel := context.WithTimeout(rootCtx, rpcTimeout)
		keyID, err := client.NewKeyPair(ctx)
		if err != nil {
			t.Fatalf("could not generate key: %v", err)
		}

		pubKey, err := client.PublicKey(ctx, keyID)
		if err != nil {
			t.Fatalf("could not fetch public key: %v", err)
		}

		crit := whisper.Criteria{
			PrivateKeyID: keyID,
		}

		filter, err := client.NewMessageFilter(ctx, crit)
		if err != nil {
			t.Fatalf("could not create subscription")
		}

		clientContexts[i] = &clientContext{
			client:    client,
			pubKeyID:  keyID,
			pubKey:    pubKey,
			nMessages: 50 + rand.Intn(100),
			filter:    filter,
			received:  make(map[string]bool),
			send:      make(map[string]bool),
		}

		expectedToReceive += clientContexts[i].nMessages

		cancel()
	}

	// cleanup filters afterwards
	defer func() {
		for _, clientContext := range clientContexts {
			if err := clientContext.client.DeleteMessageFilter(context.Background(), clientContext.filter); err != nil {
				t.Errorf("could not delete message filter: %v [%s]", err, clientContext.filter)
			}
		}
	}()

	// wait for messages
	var result = make(chan error)

	// wait for messages
	go func() {
		for i := 0; i < 30; i++ {
			nReceived := 0
			for _, clientContext := range clientContexts {
				msgs, err := clientContext.client.FilterMessages(context.Background(), clientContext.filter)
				if err != nil {
					result <- err
					return
				}
				for _, msg := range msgs {
					clientContext.received[string(msg.Payload)] = true
				}
				nReceived += len(clientContext.received)
			}

			if nReceived == expectedToReceive {
				close(result)
				return
			}

			time.Sleep(time.Second)
		}
		result <- fmt.Errorf("didn't receive all messages within timeout")
	}()

	// start sending messages to 2 other clients
	for i := 0; i < len(clientContexts); i++ {
		go func(idx int) {
			me := clientContexts[idx]
			A := clientContexts[(idx+1)%len(clientContexts)]
			B := clientContexts[(idx+2)%len(clientContexts)]

			// send a bunch of messages
			for j := 0; j < me.nMessages; j++ {
				payload := fmt.Sprintf("%x:%d", me.pubKey[:4], j)
				msg := whisper.NewMessage{
					Payload:   []byte(payload),
					PowTarget: 0.25,
					PowTime:   5,
				}

				if rand.Int31n(2) == 0 {
					A.mu.Lock()
					A.send[payload] = true
					A.mu.Unlock()
					msg.PublicKey = A.pubKey
				} else {
					B.mu.Lock()
					B.send[payload] = true
					B.mu.Unlock()
					msg.PublicKey = B.pubKey
				}

				ctx, cancel := context.WithTimeout(rootCtx, rpcTimeout)
				if err := me.client.Post(ctx, msg); err != nil {
					result <- err
					return
				}

				cancel()
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	if err := <-result; err != nil {
		t.Fatalf("test failed: %v", err)
	}

	// ensure that the received payload is correct
	for i, ctx := range clientContexts {
		if !reflect.DeepEqual(ctx.send, ctx.received) {
			t.Errorf("received invalid payloads for ctx[%d]", i)
			for key, _ := range ctx.received {
				t.Logf("%s", key)
			}
			t.Log("-------------------------")
			for key, _ := range ctx.send {
				t.Logf("%s", key)
			}
			t.Log("")
		}
	}
}

// publicKeyTest simulates 3 whisper clients sending messages to each other
// using public key encryption. This tests ensures no messages are lost and
// the payload of received messages is as expected. It uses a subscription
// to receive messages.
func publicKeyTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	type clientContext struct {
		client   *TestClient
		pubKeyID string
		pubKey   []byte
		mu       sync.Mutex
		send     map[string]bool
		received map[string]bool

		messages  chan *whisper.Message
		nMessages int
		sub       ethereum.Subscription
	}

	var (
		rootCtx        = context.Background()
		clientContexts = make([]*clientContext, len(clients))
		sendMu         sync.Mutex
		totalSend      = 0
	)

	// generate client context
	for i, client := range clients[:3] {
		ctx, cancel := context.WithTimeout(rootCtx, rpcTimeout)
		keyID, err := client.NewKeyPair(ctx)
		if err != nil {
			t.Fatalf("could not generate key: %v", err)
		}

		pubKey, err := client.PublicKey(ctx, keyID)
		if err != nil {
			t.Fatalf("could not fetch public key: %v", err)
		}

		crit := whisper.Criteria{
			PrivateKeyID: keyID,
		}
		messages := make(chan *whisper.Message)

		sub, err := client.SubscribeMessages(ctx, crit, messages)
		if err != nil {
			t.Fatalf("could not create subscription")
		}
		defer sub.Unsubscribe()

		clientContexts[i] = &clientContext{
			client:    client,
			pubKeyID:  keyID,
			pubKey:    pubKey,
			messages:  messages,
			nMessages: 50 + rand.Intn(100),
			sub:       sub,
			received:  make(map[string]bool),
			send:      make(map[string]bool),
		}

		cancel()
	}

	// wait for messages
	var result = make(chan error)

	// wait for messages
	go func() {
		for {
			select {
			case msg := <-clientContexts[0].messages:
				clientContexts[0].received[string(msg.Payload)] = true
			case msg := <-clientContexts[1].messages:
				clientContexts[1].received[string(msg.Payload)] = true
			case msg := <-clientContexts[2].messages:
				clientContexts[2].received[string(msg.Payload)] = true
			case err := <-clientContexts[0].sub.Err():
				result <- err
				return
			case err := <-clientContexts[1].sub.Err():
				result <- err
				return
			case err := <-clientContexts[2].sub.Err():
				result <- err
				return
			}
			if len(clientContexts[0].received)+len(clientContexts[1].received)+len(clientContexts[2].received) == clientContexts[0].nMessages+clientContexts[1].nMessages+clientContexts[2].nMessages {
				close(result)
				return
			}
		}
	}()

	// start sending messages to 2 other clients
	for i := 0; i < len(clientContexts); i++ {
		go func(idx int) {
			me := clientContexts[idx]
			A := clientContexts[(idx+1)%len(clientContexts)]
			B := clientContexts[(idx+2)%len(clientContexts)]

			// send a bunch of messages
			for j := 0; j < me.nMessages; j++ {
				payload := fmt.Sprintf("%x:%d", me.pubKey[:4], j)
				msg := whisper.NewMessage{
					Payload:   []byte(payload),
					PowTarget: 0.25,
					PowTime:   5,
				}

				if rand.Int31n(2) == 0 {
					A.mu.Lock()
					A.send[payload] = true
					A.mu.Unlock()
					msg.PublicKey = A.pubKey
				} else {
					B.mu.Lock()
					B.send[payload] = true
					B.mu.Unlock()
					msg.PublicKey = B.pubKey
				}

				ctx, cancel := context.WithTimeout(rootCtx, rpcTimeout)
				if err := me.client.Post(ctx, msg); err != nil {
					result <- err
					return
				}

				cancel()
				time.Sleep(50 * time.Millisecond)
			}

			sendMu.Lock()
			totalSend += me.nMessages
			sendMu.Unlock()
		}(i)
	}

	if err := <-result; err != nil {
		t.Fatalf("test failed: %v", err)
	}

	// ensure that the received payload is correct
	for i, ctx := range clientContexts {
		if !reflect.DeepEqual(ctx.send, ctx.received) {
			t.Errorf("received invalid payloads for ctx[%d]", i)
			for key, _ := range ctx.received {
				t.Logf("%s", key)
			}
			t.Log("-------------------------")
			for key, _ := range ctx.send {
				t.Logf("%s", key)
			}
			t.Log("")
		}
	}
}

// publicKeyWithTopicsAndPollingTest simulates 3 whisper clients sending
// messages to each other using public key encryption. This tests ensures
// no messages are lost and the payload of received messages is as expected.
// It uses polling for messages.
func publicKeyWithTopicsAndPollingTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	var (
		topics  = randTopics(1)
		clientA = clients[0]
		clientB = clients[1]
		clientC = clients[2]
	)

	// generate keys
	keyIDA, pubKeyA, _ := generateAsymKey(t, clientA)
	keyIDB, pubKeyB, _ := generateAsymKey(t, clientB)
	keyIDC, pubKeyC, _ := generateAsymKey(t, clientC)

	// create filters
	filterA := createMessageFilter(t, clientA, whisper.Criteria{PrivateKeyID: keyIDA, Topics: topics})
	filterB := createMessageFilter(t, clientB, whisper.Criteria{PrivateKeyID: keyIDB, Topics: topics})
	filterC := createMessageFilter(t, clientC, whisper.Criteria{PrivateKeyID: keyIDC, Topics: topics})

	// cleanup filters
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*rpcTimeout)
		if err := clientA.DeleteMessageFilter(ctx, filterA); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		if err := clientB.DeleteMessageFilter(ctx, filterB); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		if err := clientC.DeleteMessageFilter(ctx, filterC); err != nil {
			t.Errorf("could not delete message filter: %v", err)
		}
		cancel()
	}()

	// send a bunch of messages to each other
	for i := 1; i < 5; i++ {
		// prepare 2 messages, each one for the other node than the node
		// that is randomly selected to post to.
		msgs := []whisper.NewMessage{{
			Topic:     topics[0],
			PowTarget: whisper.DefaultMinimumPoW,
			PowTime:   5,
		},
			{
				Topic:     topics[0],
				PowTarget: whisper.DefaultMinimumPoW,
				PowTime:   5,
			}}

		// randomly select a client for posting the messages and prepare messages to post
		var (
			source    *TestClient
			targets   = make([]*TestClient, 2)
			filterIDs = make([]string, 2)
		)

		switch rand.Int31n(3) {
		case 0:
			source = clientA
			targets[0], targets[1], filterIDs[0], filterIDs[1] = clientB, clientC, filterB, filterC
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from A: %d", i)), pubKeyB
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from A: %d", i)), pubKeyC
		case 1:
			source = clientB
			targets[0], targets[1], filterIDs[0], filterIDs[1] = clientC, clientA, filterC, filterA
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from B: %d", i)), pubKeyC
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from B: %d", i)), pubKeyA
		case 2:
			source = clientC
			targets[0], targets[1], filterIDs[0], filterIDs[1] = clientA, clientB, filterA, filterB
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from C: %d", i)), pubKeyA
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from C: %d", i)), pubKeyB
		}

		// post messages
		for _, msg := range msgs {
			ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
			if err := source.Post(ctx, msg); err != nil {
				t.Fatalf("could not post message: %v", err)
			}
			cancel()
		}

		// poll nodes for new messages
		var wg sync.WaitGroup
		poller := func(t *testing.T, client *TestClient, filter string, expected whisper.NewMessage) {
			defer wg.Done()

			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
				received, err := client.FilterMessages(ctx, filter)
				cancel()

				if err != nil {
					t.Fatalf("could not poll for new messages: %v", err)
				}

				if len(received) > 0 {
					msg := received[0]
					if expected.Topic != msg.Topic {
						t.Errorf("unexpected topic, want %x, got %x", expected.Topic, msg.Topic)
					}
					if bytes.Compare(expected.Payload, msg.Payload) != 0 {
						t.Errorf("unexpected payload, want %x, got %x", expected.Payload, msg.Payload)
					}
					return
				}

				<-ticker.C
			}

			t.Errorf("didn't receive message within reasonable time")
		}

		wg.Add(2)
		go poller(t, targets[0], filterIDs[0], msgs[0])
		go poller(t, targets[1], filterIDs[1], msgs[1])
		wg.Wait()
	}
}

// publicKeyWithTopicsAndSubscriptionsTest simultes 3 whisper clients broadcasting
// encrypted messages with particular topics using public key encryption. This tests
// ensures no messages are lost and the payload of received messages is as expected.
func publicKeyWithTopicsAndSubscriptionsTest(t *testing.T, clients []*TestClient) {
	t.Parallel()

	var (
		topics  = randTopics(1)
		clientA = clients[0]
		clientB = clients[1]
		clientC = clients[2]
	)

	// generate keys
	keyIDA, pubKeyA, _ := generateAsymKey(t, clientA)
	keyIDB, pubKeyB, _ := generateAsymKey(t, clientB)
	keyIDC, pubKeyC, _ := generateAsymKey(t, clientC)

	// create subscription that will emit a message when the shared password was
	// used to encrypt the message.
	subA, inA := createMessageSubscription(t, clientA, whisper.Criteria{PrivateKeyID: keyIDA, Topics: topics})
	subB, inB := createMessageSubscription(t, clientB, whisper.Criteria{PrivateKeyID: keyIDB, Topics: topics})
	subC, inC := createMessageSubscription(t, clientC, whisper.Criteria{PrivateKeyID: keyIDC, Topics: topics})

	defer subA.Unsubscribe()
	defer subB.Unsubscribe()
	defer subC.Unsubscribe()

	for i := 0; i < 50; i++ {
		// prepare 2 messages, each one for the other node than the node
		// that is randomly selected to post to.
		msgs := []whisper.NewMessage{{
			Topic:     topics[0],
			PowTarget: whisper.DefaultMinimumPoW,
			PowTime:   5,
		},
			{
				Topic:     topics[0],
				PowTarget: whisper.DefaultMinimumPoW,
				PowTime:   5,
			}}

		// randomly select a client for posting the messages and prepare messages to post
		var client *TestClient
		switch rand.Int31n(3) {
		case 0:
			client = clientA
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from A: %d", i)), pubKeyB
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from A: %d", i)), pubKeyC
		case 1:
			client = clientB
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from B: %d", i)), pubKeyC
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from B: %d", i)), pubKeyA
		case 2:
			client = clientC
			msgs[0].Payload, msgs[0].PublicKey = []byte(fmt.Sprintf("from C: %d", i)), pubKeyA
			msgs[1].Payload, msgs[1].PublicKey = []byte(fmt.Sprintf("from C: %d", i)), pubKeyB
		}

		for _, msg := range msgs {
			ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
			if err := client.Post(ctx, msg); err != nil {
				t.Fatalf("could not post message: %v", err)
			}
			cancel()
		}

		// wait for messages to arrive
		var (
			msgA *whisper.Message
			msgB *whisper.Message
			msgC *whisper.Message
		)

		timeout := time.NewTimer(10 * time.Second)
		nReceived := 0
		for nReceived != 2 {
			select {
			case msgA = <-inA:
				if bytes.Compare(msgs[0].Payload, msgA.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msgs[0].Payload, msgA.Payload)
				}
			case msgB = <-inB:
				if bytes.Compare(msgs[0].Payload, msgB.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msgs[0].Payload, msgB.Payload)
				}
			case msgC = <-inC:
				if bytes.Compare(msgs[0].Payload, msgC.Payload) != 0 {
					t.Fatalf("unexpected playload, want: %x, got: %x", msgs[0].Payload, msgC.Payload)
				}
			case err := <-subA.Err():
				t.Fatalf("subscription returned err: %v", err)
			case err := <-subB.Err():
				t.Fatalf("subscription returned err: %v", err)
			case err := <-subC.Err():
				t.Fatalf("subscription returned err: %v", err)
			case <-timeout.C:
				t.Fatal("didn't receive message within deadline")
			}
			nReceived++
		}
	}
}

// randTopics is a helper function that generates random topics.
// n specifies the number of topics to generate.
func randTopics(n int) []whisper.TopicType {
	t := make([]whisper.TopicType, n)
	for i := 0; i < n; i++ {
		var topic whisper.TopicType
		rand.Read(topic[:])
		t[i] = topic
	}
	return t
}

// enode is a helper function that returns the enode for the given client.
// in case of an error t.Fatalf is called.
func enode(t *testing.T, client *TestClient) string {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	var nodeInfo p2p.NodeInfo
	if err := client.CallContext(ctx, &nodeInfo, "admin_nodeInfo"); err != nil {
		t.Fatalf("could not fetch enode: %v", err)
	}
	cancel()
	return nodeInfo.Enode
}

// directP2PMessagingTest sends a message direct to a peer and by passing
// common checks such as PoW or message size. It uses a message filter for
// polling for new messages.
func directP2PMessagingWithPollingTest(t *testing.T, clients []*TestClient) {
	clientA := clients[0]
	clientB := clients[1]
	enodeA := enode(t, clientA)
	enodeB := enode(t, clientB)

	// generate keys
	keyIDB, pubKeyB, _ := generateAsymKey(t, clientB)

	// create subscription in clientB
	//filterA := createMessageFilter(t, clientA, whisper.Criteria{SymKeyID: keyA, Topics: topics})
	filter := createMessageFilter(t, clientB, whisper.Criteria{PrivateKeyID: keyIDB, AllowP2P: true})
	defer func() {
		if err := clientB.DeleteMessageFilter(context.Background(), filter); err != nil {
			t.Errorf("could not remove message filter: %v", err)
		}
	}()

	// send a direct message from A to B with too low PoW and too big payload (should be rejected)
	msg := whisper.NewMessage{
		Payload:   bytes.Repeat([]byte{0x40}, 100),
		PublicKey: pubKeyB,
		PowTarget: whisper.DefaultMinimumPoW - 0.01,
		PowTime:   5,
	}

	// post with too low PoW
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientA.Post(ctx, msg); err == nil {
		t.Fatalf("message should have been rejected")
	}
	cancel()

	// mark peer as trusted
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientB.MarkTrustedPeer(ctx, enodeA); err != nil {
		t.Errorf("could not mark peer %s as trusted: %v", enodeA, err)
	}
	cancel()

	// send message again and this time it should be accepted since it is send
	// to a peer direct instead of broadcasted onto the network and from a peer
	// that is trusted.
	msg.TargetPeer = enodeB
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientA.Post(ctx, msg); err != nil {
		t.Fatalf("unable to post message: %v", err)
	}
	cancel()

	// receive message within 30s
	for i := 0; i < 30; i++ {
		messages, err := clientB.FilterMessages(context.Background(), filter)
		if err != nil {
			t.Errorf("error receiving messages: %v", err)
		}

		if len(messages) > 0 {
			return // expected
		}

		time.Sleep(time.Second)
	}
}

// directP2PMessagingTest sends a message direct to a peer and by passing
// common checks such as PoW or message size.
func directP2PMessagingTest(t *testing.T, clients []*TestClient) {
	clientA := clients[0]
	clientB := clients[1]
	enodeA := enode(t, clientA)
	enodeB := enode(t, clientB)

	// generate keys
	keyIDB, pubKeyB, _ := generateAsymKey(t, clientB)

	// create subscription in clientB
	subB, inB := createMessageSubscription(t, clientB, whisper.Criteria{PrivateKeyID: keyIDB, AllowP2P: true})
	defer subB.Unsubscribe()

	// send a direct message from A to B with too low PoW and too big payload (should be rejected)
	msg := whisper.NewMessage{
		Payload:   bytes.Repeat([]byte{0x40}, 100),
		PublicKey: pubKeyB,
		PowTarget: whisper.DefaultMinimumPoW - 0.01,
		PowTime:   5,
	}

	// post with too low PoW
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientA.Post(ctx, msg); err == nil {
		t.Fatalf("message should have been rejected")
	}
	cancel()

	// mark peer as trusted
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientB.MarkTrustedPeer(ctx, enodeA); err != nil {
		t.Errorf("could not mark peer %s as trusted: %v", enodeA, err)
	}
	cancel()

	// send message again and this time it should be accepted since it is send
	// to a peer direct instead of broadcasted onto the network and from a peer
	// that is trusted.
	msg.TargetPeer = enodeB
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientA.Post(ctx, msg); err != nil {
		t.Fatalf("unable to post message: %v", err)
	}
	cancel()

	// receive message
	timeout := time.NewTimer(10 * time.Second)
	select {
	case <-inB:
		// expected
	case err := <-subB.Err():
		t.Fatalf("received error: %v", err)
	case <-timeout.C:
		t.Fatalf("didn't receive message")
	}
}

// trustedPeerTest currently only tests if a valid enode can be used.
// hive simulations run all on a local host which means that messages
// are received within milliseconds. This makes testing expired message
// delivery impossible.
func trustedPeerTest(t *testing.T, clients []*TestClient) {
	clientA := clients[0]
	clientB := clients[1]
	enodeA := enode(t, clientA)

	// mark peer as trusted
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientB.MarkTrustedPeer(ctx, enodeA); err != nil {
		t.Errorf("could not mark peer %s as trusted: %v", enodeA, err)
	}
	cancel()

	// mark peer as trusted with an invalid enode
	invalidEnode := enodeA + "a"
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientB.MarkTrustedPeer(ctx, invalidEnode); err == nil {
		t.Errorf("marking peer with invalid enode should have failed")
	}
	cancel()

	// marking node's own enode as trusted should fail
	enodeB := enode(t, clientB)
	ctx, cancel = context.WithTimeout(context.Background(), rpcTimeout)
	if err := clientB.MarkTrustedPeer(ctx, enodeB); err == nil {
		t.Errorf("marking peer with own enode should fail")
	}
	cancel()
}
