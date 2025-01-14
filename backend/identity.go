package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	swie "github.com/spruceid/siwe-go"
)

var ApocryphS3Scheme string = "apocryph-s3"
var SwieDomain string = "localhost:5173" // 's3.apocryph.io'

type AuthenticationFailure struct {
	Reason string `json:"reason"`
}

type AuthenticationResult struct {
	User               string                 `json:"user"`
	MaxValiditySeconds int                    `json:"maxValiditySeconds"`
	Claims             map[string]interface{} `json:"claims"`
}

type Token struct {
	Message   string `json:"message"`
	Signature string `json:"signature"`
}

type identityServer struct {
	replicationPublicKeyAddress common.Address
}

func RunIdentityServer(ctx context.Context, serveAddress string, replicationPublicKeyAddress common.Address) error {
	server := identityServer{
		replicationPublicKeyAddress: replicationPublicKeyAddress,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /", server.authenticateHandler)
	s := &http.Server{
		Addr:    serveAddress,
		Handler: mux,
	}
	go func() {
		<-ctx.Done()
		err := s.Shutdown(context.TODO())
		log.Println(err)
	}()

	log.Println("Identity plugin provider listening on ", serveAddress)
	err := s.ListenAndServe()
	log.Println(err)

	return err
}
func (s identityServer) authenticateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	result, err := s.authenticateHelper(r.URL.Query().Get("token"))
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(AuthenticationFailure{
			Reason: err.Error(),
		})
	} else {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	}
}

func (s identityServer) authenticateHelper(tokenString string) (result AuthenticationResult, err error) {
	token := &Token{}
	err = json.Unmarshal([]byte(tokenString), token)
	if err != nil {
		return
	}
	message, err := swie.ParseMessage(token.Message)
	if err != nil {
		return
	}
	log.Printf("Received SWIE message: %s\n", message.String())

	_, err = message.Verify(token.Signature, &SwieDomain, nil, nil)
	if err != nil {
		return
	}

	var bucketId string
	var scope string

	for _, resource := range message.GetResources() {
		if resource.Scheme == ApocryphS3Scheme {
			if bucketId != "" {
				err = fmt.Errorf("Multiple resource claims for different buckets!")
				return
			}
			bucketId = resource.Host
		}
	}

	address := message.GetAddress()
	if address == s.replicationPublicKeyAddress {
		if bucketId == "" {
			err = fmt.Errorf("Expected resource claim in special message from the replication system address!")
			return
		}
		// TODO: All bucket ids are allowed here, for now, without checking if the message is coming from the expected replica
		scope = "remoteReplicant"
	} else {
		expectedBucketId := hex.EncodeToString(address[:])
		if bucketId == "" {
			bucketId = expectedBucketId
		}
		if bucketId != expectedBucketId {
			err = fmt.Errorf("Invalid bucket specified in resources!")
			return
		}
		scope = ""
	}

	log.Printf("Bucket is %s; role: %s\n", bucketId, scope)
	result = AuthenticationResult{
		User:               address.Hex(),
		MaxValiditySeconds: 3600, // token.ExpirationTime.Unix() - time.Now().Unix()
		Claims: map[string]interface{}{
			"preferred_username": bucketId,
			"scope":              []string{scope},
		},
	}
	return
}

type TokenSigner struct {
	privateKey *ecdsa.PrivateKey
}

func NewTokenSigner(privateKeyString string) (*TokenSigner, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyString)
	if err != nil {
		return nil, err
	}
	return &TokenSigner{
		privateKey: privateKey,
	}, nil
}

func (s *TokenSigner) GetPublicAddress() common.Address {
	return crypto.PubkeyToAddress(s.privateKey.PublicKey)
}

func (s *TokenSigner) GetReplicationToken(bucketId string) (token string, err error) {
	message, err := swie.InitMessage(
		SwieDomain,
		"localhost",
		"localhost",
		swie.GenerateNonce(),
		map[string]interface{}{
			"issuedAt":       time.Now(),
			"expirationTime": time.Now().Add(time.Minute * 10),
			"resources": []url.URL{
				{Scheme: ApocryphS3Scheme, Host: bucketId},
			},
		},
	)
	if err != nil {
		return
	}

	// Ref: swie.Message.eip191Hash
	messageBytes := []byte(message.String())
	messageEip191 := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(messageBytes), messageBytes)
	messageHash := crypto.Keccak256Hash([]byte(messageEip191))

	signatureBytes, err := crypto.Sign(messageHash.Bytes(), s.privateKey)
	if err != nil {
		return
	}

	tokenBytes, err := json.Marshal(Token{
		Message:   message.String(),
		Signature: hexutil.Encode(signatureBytes),
	})
	if err != nil {
		return
	}

	token = string(tokenBytes)
	return
}
