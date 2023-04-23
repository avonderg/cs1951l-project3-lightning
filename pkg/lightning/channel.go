package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"Coin/pkg/utils"
	"fmt"
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
	channel := &Channel{Funder: true, CounterPartyPubKey: theirPubKey, State: 0, MyTransactions: []*block.Transaction{}, TheirTransactions: []*block.Transaction{}, MyRevocationKeys: map[string][]byte{}, TheirRevocationKeys: map[string]*RevocationInfo{}}
	ln.Channels[peer] = channel
	req := WalletRequest{Amount: amount, Fee: 2 * fee, CounterPartyPubKey: theirPubKey}
	fund_tx := ln.generateFundingTransaction(req)
	pub, priv := GenerateRevocationKey()
	channel.MyRevocationKeys[fund_tx.Hash()] = priv
	refund_tx := ln.generateRefundTransaction(theirPubKey, fund_tx, fee, pub)
	channelRq := &pro.OpenChannelRequest{Address: ln.Address, PublicKey: ln.Id.GetPublicKeyBytes(), FundingTransaction: block.EncodeTransaction(fund_tx), RefundTransaction: block.EncodeTransaction(refund_tx)}
	response, _ := peer.Addr.OpenChannelRPC(channelRq)
	channel.FundingTransaction = block.DecodeTransaction(response.SignedFundingTransaction)
	channel.TheirTransactions = append(channel.TheirTransactions, block.DecodeTransaction(response.SignedRefundTransaction))
	channel.MyTransactions = append(channel.MyTransactions, block.DecodeTransaction(response.SignedRefundTransaction))
	signed, _ := utils.Sign(ln.Id.GetPrivateKey(), []byte(fund_tx.Hash()))
	fund_tx.Witnesses = append(fund_tx.Witnesses, signed)
}

// UpdateState is called to update the state of a channel.
func (ln *LightningNode) UpdateState(peer *peer.Peer, tx *block.Transaction) {
	// TODO
	tx_a := &pro.TransactionWithAddress{Transaction: block.EncodeTransaction(tx), Address: peer.Addr.Addr}
	fmt.Sprint("got here")
	updated_tx, _ := peer.Addr.GetUpdatedTransactionsRPC(tx_a)
	channel := ln.Channels[peer]
	channel.MyTransactions = append(channel.MyTransactions, block.DecodeTransaction(updated_tx.SignedTransaction))
	ln.SignTransaction(block.DecodeTransaction(updated_tx.UnsignedTransaction))
	channel.TheirTransactions = append(channel.TheirTransactions, block.DecodeTransaction(updated_tx.UnsignedTransaction))
	revKey := channel.MyRevocationKeys[tx.Hash()]
	signed := &pro.SignedTransactionWithKey{SignedTransaction: updated_tx.SignedTransaction, Address: peer.Addr.Addr, RevocationKey: revKey}
	key, _ := peer.Addr.GetRevocationKeyRPC(signed)
	ln.UpdateState(peer, tx)
	index := 0
	if channel.Funder {
		index = 1
	}
	c := updated_tx.SignedTransaction.Outputs[index]
	scriptType, _ := script.DetermineScriptType(c.LockingScript)
	hash := tx.Hash()
	revInfo := &RevocationInfo{RevKey: key.Key, TransactionOutput: block.DecodeTransactionOutput(c), OutputIndex: uint32(index), TransactionHash: hash, ScriptType: scriptType}

	channel.TheirRevocationKeys[hash] = revInfo

	//key := channel.MyRevocationKeys[revInfo.TransactionHash]
	//rKey := &pro.RevocationKey{Key: key}
}
