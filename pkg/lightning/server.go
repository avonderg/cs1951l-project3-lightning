package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/block"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"context"
	"time"
)

// Version was copied directly from pkg/server.go. Only changed the function receiver and types
func (ln *LightningNode) Version(ctx context.Context, in *pro.VersionRequest) (*pro.Empty, error) {
	// Reject all outdated versions (this is not true to Satoshi Client)
	if in.Version != ln.Config.Version {
		return &pro.Empty{}, nil
	}
	// If addr map is full or does not contain addr of ver, reject
	newAddr := address.New(in.AddrMe, uint32(time.Now().UnixNano()))
	if ln.AddressDB.Get(newAddr.Addr) != nil {
		err := ln.AddressDB.UpdateLastSeen(newAddr.Addr, newAddr.LastSeen)
		if err != nil {
			return &pro.Empty{}, nil
		}
	} else if err := ln.AddressDB.Add(newAddr); err != nil {
		return &pro.Empty{}, nil
	}
	newPeer := peer.New(ln.AddressDB.Get(newAddr.Addr), in.Version, in.BestHeight)
	// Check if we are waiting for a ver in response to a ver, do not respond if this is a confirmation of peering
	pendingVer := newPeer.Addr.SentVer != time.Time{} && newPeer.Addr.SentVer.Add(ln.Config.VersionTimeout).After(time.Now())
	if ln.PeerDb.Add(newPeer) && !pendingVer {
		newPeer.Addr.SentVer = time.Now()
		_, err := newAddr.VersionRPC(&pro.VersionRequest{
			Version:    ln.Config.Version,
			AddrYou:    in.AddrYou,
			AddrMe:     ln.Address,
			BestHeight: ln.BlockHeight,
		})
		if err != nil {
			return &pro.Empty{}, err
		}
	}
	return &pro.Empty{}, nil
}

// OpenChannel is called by another lightning node that wants to open a channel with us
func (ln *LightningNode) OpenChannel(ctx context.Context, in *pro.OpenChannelRequest) (*pro.OpenChannelResponse, error) {
	//TODO
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil { // is not in the PeerDB
		return nil, nil
	}
	if _, ok := ln.Channels[peer]; ok { // if there's already a channel opened
		return nil, nil
	}
	funding := block.DecodeTransaction(in.FundingTransaction)
	refund := block.DecodeTransaction(in.RefundTransaction)
	ln.ValidateAndSign(funding)
	ln.ValidateAndSign(refund)

	// Generate a dummy revocation key pair and add it to MyRevocationKeys
	pub, priv := GenerateRevocationKey()
	m := make(map[string][]byte)
	m[string(pub)] = priv
	channel := &Channel{FundingTransaction: funding, TheirTransactions: []*block.Transaction{funding}, MyTransactions: []*block.Transaction{funding}, MyRevocationKeys: m}

	ln.Channels[peer] = channel

	// Construct and sign our response
	resp := &pro.OpenChannelResponse{
		PublicKey:                pub,
		SignedFundingTransaction: block.EncodeTransaction(funding),
		SignedRefundTransaction:  block.EncodeTransaction(refund)}
	return resp, nil
}

func (ln *LightningNode) GetUpdatedTransactions(ctx context.Context, in *pro.TransactionWithAddress) (*pro.UpdatedTransactions, error) {
	// TODO
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil { // is not in the PeerDB
		return nil, nil
	}
	tx := block.DecodeTransaction(in.Transaction)
	//ln.ValidateAndSign(tx)
	signature, _ := tx.Sign(ln.Id)
	tx.Witnesses = append(tx.Witnesses, signature)

	public, private := GenerateRevocationKey()

	toSign := ln.generateTransactionWithCorrectScripts(peer, tx, public)

	channel := ln.Channels[peer]
	channel.MyRevocationKeys[string(public)] = private // add to map
	channel.TheirTransactions = append(channel.TheirTransactions, toSign)

	newTx := &pro.UpdatedTransactions{SignedTransaction: block.EncodeTransaction(tx), UnsignedTransaction: block.EncodeTransaction(toSign)}

	return newTx, nil
}

func (ln *LightningNode) GetRevocationKey(ctx context.Context, in *pro.SignedTransactionWithKey) (*pro.RevocationKey, error) {
	// TODO
	peer := ln.PeerDb.Get(in.Address)
	if peer == nil { // is not in the PeerDB
		return nil, nil
	}
	channel := ln.Channels[peer]
	channel.MyTransactions = append(channel.MyTransactions, block.DecodeTransaction(in.SignedTransaction))

	scriptType, _ := script.DetermineScriptType(in.RevocationKey)

	var c *pro.TransactionOutput
	var index uint32

	if channel.Funder == true {
		c = in.SignedTransaction.Outputs[0]
		index = 0
	} else {
		c = in.SignedTransaction.Outputs[1]
		index = 1
	}
	hash := string(c.LockingScript)
	revInfo := &RevocationInfo{RevKey: in.RevocationKey, TransactionOutput: block.DecodeTransactionOutput(c), OutputIndex: uint32(index), TransactionHash: hash, ScriptType: scriptType}

	channel.TheirRevocationKeys[hash] = revInfo

	return nil, nil
}
