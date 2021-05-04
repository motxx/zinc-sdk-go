package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// CGO_LDFLAGS="-L./libs" go build

/*

  "transaction": {
    "tx": {
      "type": "Transfer",
      "accountId": 1,
      "from": "0x36615cf349d7f6344891b1e7ca7c72883f5dc049",
      "to": "0x1234567812345678123456781234567812345678",
      "token": 0,
      "amount": "0",
      "fee": "37500000000000",
      "nonce": 2,
      "signature": {
        "pubKey": "07f86efb9bf58d5ebf23042406cb43e9363879ff79223be05b7feac1dbc58c86",
        "signature": "042c7356c3970c5ab620e1eaf0a9e39563edc9383072ac33a29398f11678b2a3acdc40ff05acd225b6a71962cfabfa6012fae8492106987bcd48135fefa09c02"
      }
    },
    "ethereumSignature": {
      "type": "EthereumSignature",
      "signature": "0xbe7a011c0b03a2ab8eceb3f51ec3055e5998b025e3e41a320f6b00532a4c49604608fe7b9c36d837c36817bbaf5570197484281dd45d83f2d9ef867b7454b91e1b"
    }
  }

*/
const (
	MAX_NUMBER_OF_ACCOUNTS = 16777216 // math.Pow(2, 24)
	MAX_NUMBER_OF_TOKENS   = 128
)

type ContractInput struct {
	Arguments   interface{} `json:"arguments"`
	Transaction Transaction `json:"transaction"`
}

type Transaction struct {
	Tx           Tx                `json:"tx"`
	EthSignature EthereumSignature `json:"ethereumSignature"`
}

type Tx struct {
	Type       string        `json:"type"`
	AccountId  uint64        `json:"accountId"`
	From       string        `json:"from"`
	To         string        `json:"to"`
	Token      uint64        `json:"token"`
	Amount     string        `json:"amount"`
	Fee        string        `json:"fee"`
	Nonce      uint64        `json:"nonce"`
	Signature  Signature     `json:"signature"`
	ValidFrom  time.Duration `json:"validFrom'`
	ValidUntil time.Duration `json:"validUntil"`
}

type Signature struct {
	PubKey    string `json:"pubKey"`
	Signature string `json:"signature"`
}

type EthereumSignature struct {
	Type      string `json:"type"`
	Signature string `json:"signature"`
}

// Uint2bytes converts uint64 to []byte
// https://qiita.com/ryskiwt/items/17617d4f3e8dde7c2b8e
func Uint2bytes(i uint64, size int) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, i)
	return bytes[8-size : 8]
}

func serializeAccountId(id uint64) ([]byte, error) {
	if id >= MAX_NUMBER_OF_ACCOUNTS {
		return nil, fmt.Errorf("AccountId is too big")
	}
	return Uint2bytes(id, 4), nil
}

func removeAddressPrefix(address string) (string, error) {
	if strings.HasPrefix(address, "0x") {
		return address[2:], nil
	}
	if strings.HasPrefix(address, "sync:") {
		return address[5:], nil
	}
	return "", fmt.Errorf("ETH address must start with '0x' and PubKeyHash must start with 'sync:'")
}

// Arrayify hex string address to byte array
// https://github.com/ethers-io/ethers.js/blob/4898e7baacc4ed40d880b48e894b61118776dddb/packages/bytes/src.ts/index.ts#L112-L130
func arrayifyAddress(address string) ([]byte, error) {
	var res []byte
	for i := 0; i < len(address); i += 2 {
		s := "0x" + string(address[i]) + string(address[i+1])
		val, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return nil, err
		}
		res = append(res, byte(val))
	}
	return res, nil
}

func serializeAddress(address string) ([]byte, error) {
	prefixless, err := removeAddressPrefix(address)
	if err != nil {
		return nil, err
	}
	//	bytes := []byte("0x" + prefixless)
	bytes, err := arrayifyAddress(prefixless)
	if err != nil {
		return nil, err
	}
	if len(bytes) != 20 {
		return nil, fmt.Errorf("Address must be 20 bytes long. len: %d", len(bytes))
	}
	return bytes, nil
}

func serializeTokenId(tokenId uint64) ([]byte, error) {
	if tokenId >= MAX_NUMBER_OF_TOKENS {
		return nil, fmt.Errorf("TokenId is too big")
	}
	return Uint2bytes(tokenId, 2), nil
}

func serializeAmountPacked(amount string) ([]byte, error) {
	// FIXME
	//	if (closestPackableTransactionAmount(amount.toString()).toString() !== amount.toString()) {
	//		throw new Error('Transaction Amount is not packable');
	//	}
	//	return packAmount(amount)
	return []byte(amount), nil
}

func serializeFeePacked(fee string) ([]byte, error) {
	// FIXME
	return []byte(fee), nil
}

func serializeNonce(nonce uint64) ([]byte, error) {
	return Uint2bytes(nonce, 4), nil
}

func serializeTimestamp(ts time.Duration) ([]byte, error) {
	if ts < 0 {
		return nil, fmt.Errorf("Negative timestamp")
	}
	return append(make([]byte, 4), Uint2bytes(uint64(ts), 4)...), nil
}

func serializeTransfer(tx *Tx) ([]byte, error) {
	type_ := make([]byte, 0, 5)
	accountId, err := serializeAccountId(tx.AccountId)
	if err != nil {
		return nil, err
	}
	from, err := serializeAddress(tx.From)
	if err != nil {
		return nil, err
	}
	to, err := serializeAddress(tx.To)
	if err != nil {
		return nil, err
	}
	token, err := serializeTokenId(tx.Token)
	if err != nil {
		return nil, err
	}
	amount, err := serializeAmountPacked(tx.Amount)
	if err != nil {
		return nil, err
	}
	fee, err := serializeFeePacked(tx.Fee)
	if err != nil {
		return nil, err
	}
	nonce, err := serializeNonce(tx.Nonce)
	if err != nil {
		return nil, err
	}
	validFrom, err := serializeTimestamp(tx.ValidFrom)
	if err != nil {
		return nil, err
	}
	validUntil, err := serializeTimestamp(tx.ValidUntil)
	if err != nil {
		return nil, err
	}
	res := append(type_, accountId...)
	res = append(res, from...)
	res = append(res, to...)
	res = append(res, token...)
	res = append(res, amount...)
	res = append(res, fee...)
	res = append(res, nonce...)
	res = append(res, validFrom...)
	res = append(res, validUntil...)
	return res, nil
}

func main() {
	/*
		seed := make([]byte, 32)
		message := []byte("hello")

		privateKey, err := zkscrypto.NewPrivateKey(seed)
		if err != nil {
			log.Fatalf("error creating private key: %s", err.Error())
		}
		publicKey, err := privateKey.PublicKey()
		if err != nil {
			log.Fatalf("error creating public key: %s", err.Error())
		}
		publicKeyHash, err := publicKey.Hash()
		if err != nil {
			log.Fatalf("error creating public key hash: %s", err.Error())
		}
		signature, err := privateKey.Sign(message)
		if err != nil {
			log.Fatalf("error signing message: %s", err.Error())
		}
		log.Printf("Seed: %s\n", hex.EncodeToString(seed))
		log.Printf("Private key: %s\n", privateKey.HexString())
		log.Printf("Public key: %s\n", publicKey.HexString())
		log.Printf("Public key hash: %s\n", publicKeyHash.HexString())
		log.Printf("Signature: %s\n", signature.HexString())
	*/
	bytes, err := os.ReadFile("data/input.json")
	if err != nil {
		log.Fatal(err)
	}

	var input ContractInput
	if err = json.Unmarshal(bytes, &input); err != nil {
		log.Fatal(err)
	}

	log.Printf("input: %v\n", input)

	ser, err := serializeTransfer(&input.Transaction.Tx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v\n", ser)
}
