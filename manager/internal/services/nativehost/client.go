// Package nativehost connects the Manager to the Hypervisor's fixed JSON broker.
// It never constructs shell commands from operation arguments.
package nativehost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const brokerCommand = "sudo -n /usr/libexec/vmalpha-api"
const maxResponse = 8 * 1024 * 1024

type Config struct{ Address, User, PrivateKeyFile, KnownHostsFile string }
type Client struct {
	address string
	ssh     *ssh.ClientConfig
}

// New requires an explicitly enrolled host key and a dedicated SSH identity.
// Trust-on-first-use and password authentication are deliberately unavailable.
func New(cfg Config) (*Client, error) {
	if cfg.User == "" || cfg.KnownHostsFile == "" || cfg.PrivateKeyFile == "" {
		return nil, errors.New("host enrollment identity is incomplete")
	}
	if _, _, err := net.SplitHostPort(cfg.Address); err != nil {
		return nil, errors.New("host address must include a port")
	}
	info, err := os.Stat(cfg.PrivateKeyFile)
	if err != nil {
		return nil, errors.New("host identity unavailable")
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("host private key must be a private regular file")
	}
	key, err := os.ReadFile(cfg.PrivateKeyFile)
	if err != nil {
		return nil, errors.New("host identity unavailable")
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.New("invalid host identity")
	}
	callback, err := knownhosts.New(cfg.KnownHostsFile)
	if err != nil {
		return nil, errors.New("enrolled host keys unavailable")
	}
	// Negotiate only algorithms present in the enrolled keys for this address.
	// Otherwise SSH may select a different server key before known_hosts is checked.
	keys := []ssh.PublicKey{signer.PublicKey()}
	if probeErr := callback(cfg.Address, endpointAddress(cfg.Address), signer.PublicKey()); probeErr != nil {
		var mismatch *knownhosts.KeyError
		if !errors.As(probeErr, &mismatch) || len(mismatch.Want) == 0 {
			return nil, errors.New("no trusted host key enrolled for address")
		}
		keys = nil
		for _, wanted := range mismatch.Want {
			keys = append(keys, wanted.Key)
		}
	}
	algorithms := []string{}
	seen := map[string]bool{}
	for _, key := range keys {
		names := []string{key.Type()}
		if key.Type() == ssh.KeyAlgoRSA {
			names = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
		}
		for _, name := range names {
			if !seen[name] {
				algorithms = append(algorithms, name)
				seen[name] = true
			}
		}
	}
	return &Client{cfg.Address, &ssh.ClientConfig{User: cfg.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: callback, HostKeyAlgorithms: algorithms, Timeout: 15 * time.Second}}, nil
}

type Request struct {
	Operation string         `json:"op"`
	Arguments map[string]any `json:"args"`
}
type response struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// Call returns only the broker's JSON result. Callers must authorize the operation
// and enrolled host before invoking this transport. Mutation retry is not automatic.
func (c *Client) Call(ctx context.Context, request Request) (json.RawMessage, error) {
	if request.Operation == "" {
		return nil, errors.New("operation is required")
	}
	payload, err := json.Marshal(request)
	if err != nil || len(payload) > maxResponse {
		return nil, errors.New("invalid broker request")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", c.address)
	if err != nil {
		return nil, errors.New("host connection failed")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	transport, channels, requests, err := ssh.NewClientConn(conn, c.address, c.ssh)
	if err != nil {
		return nil, fmt.Errorf("host authentication or identity verification failed: %w", err)
	}
	client := ssh.NewClient(transport, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return nil, errors.New("host broker session failed")
	}
	defer session.Close()
	var output limitedBuffer
	session.Stdin = bytes.NewReader(payload)
	session.Stdout = &output
	// Broker stderr may include sensitive operational information; never surface it.
	session.Stderr = io.Discard
	runErr := session.Run(brokerCommand)
	if ctx.Err() != nil {
		return nil, fmt.Errorf("host request interrupted: %w", ctx.Err())
	}
	if output.exceeded {
		return nil, errors.New("host response exceeds size limit")
	}
	result, err := decode(output.data.Bytes())
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return nil, errors.New("host broker execution failed")
	}
	return result, nil
}

func decode(raw []byte) (json.RawMessage, error) {
	var result response
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, errors.New("invalid host response")
	}
	if !result.OK {
		return nil, &BrokerError{}
	}
	if len(result.Result) == 0 {
		return nil, errors.New("host response omitted result")
	}
	return result.Result, nil
}

type limitedBuffer struct {
	data     bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > maxResponse-b.data.Len() {
		b.exceeded = true
		return 0, errors.New("host response exceeds size limit")
	}
	return b.data.Write(p)
}

type endpointAddress string

func (a endpointAddress) Network() string { return "tcp" }
func (a endpointAddress) String() string  { return string(a) }

// BrokerError confirms the remote command reported failure; partial changes may exist.
type BrokerError struct{}

func (*BrokerError) Error() string { return "host broker reported operation failure" }
