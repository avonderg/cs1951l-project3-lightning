package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/peer"
)

// Channel is our node's view of a channel
// Funder is whether we are the channel's funder
// FundingTransaction is the channel's funding transaction
// CounterPartyPubKey is the other node's public key
// State is the current state that we are at. On instantiation,
// the refund transaction is the transaction for state 0
// Transactions is the slice of transactions, indexed by state
// MyRevocationKeys is a mapping of my private revocation keys
// TheirRevocationKeys is a mapping of their private revocation keys
type Channel struct {
	Funder             bool
	FundingTransaction *block.Transaction
	State              int
	CounterPartyPubKey []byte

	MyTransactions    []*block.Transaction
	TheirTransactions []*block.Transaction

	MyRevocationKeys    map[string][]byte
	TheirRevocationKeys map[string]*RevocationInfo
}

type RevocationInfo struct {
	RevKey            []byte
	TransactionOutput *block.TransactionOutput
	OutputIndex       uint32
	TransactionHash   string
	ScriptType        int
}

// GenerateRevocationKey returns a new public, private key pair
func GenerateRevocationKey() ([]byte, []byte) {
	i, _ := id.CreateSimpleID()
	return i.GetPublicKeyBytes(), i.GetPrivateKeyBytes()
}

// CreateChannel creates a channel with another lightning node
// fee must be enough to cover two transactions! You will get back change from first
func (ln *LightningNode) CreateChannel(peer *peer.Peer, theirPubKey []byte, amount uint32, fee uint32) {
	// TODO
}

// UpdateState is called to update the state of a channel.
func (ln *LightningNode) UpdateState(peer *peer.Peer, tx *block.Transaction) {
	// TODO
}
