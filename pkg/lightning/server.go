package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/block"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
	"context"
	"fmt"
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
		return nil, fmt.Errorf("peer %s not found in PeerDB", in.Address)
	}
	if _, ok := ln.Channels[peer]; ok { // if there's already a channel opened
		return nil, fmt.Errorf("channel with peer %s already exists", in.Address)
	}
	funding := block.DecodeTransaction(in.GetFundingTransaction())
	refund := block.DecodeTransaction(in.GetRefundTransaction())
	err1 := ln.ValidateAndSign(funding)
	if err1 != nil {
		return nil, err1
	}
	err2 := ln.ValidateAndSign(refund)
	if err2 != nil {
		return nil, err2
	}
	channel := &Channel{Funder: false, FundingTransaction: funding, State: 0, CounterPartyPubKey: in.PublicKey, TheirTransactions: []*block.Transaction{funding}, MyTransactions: []*block.Transaction{funding}, MyRevocationKeys: make(map[string][]byte), TheirRevocationKeys: make(map[string]*RevocationInfo)}
	pub, priv := GenerateRevocationKey()
	channel.MyRevocationKeys[string(pub)] = priv
	ln.Channels[peer] = channel

	// Construct and sign our response
	resp := &pro.OpenChannelResponse{
		PublicKey:                in.GetPublicKey(),
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
	signature, err := tx.Sign(ln.Id)
	if err != nil {
		return nil, err
	}

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
	tx := block.DecodeTransaction(in.SignedTransaction)
	channel.MyTransactions = append(channel.MyTransactions, tx)

	scriptType, err := script.DetermineScriptType(in.RevocationKey)
	if err != nil {
		return nil, err
	}

	var c *block.TransactionOutput
	var index uint32

	if channel.Funder == true {
		c = tx.Outputs[1]
		index = 1
	} else {
		c = tx.Outputs[0]
		index = 0
	}
	hash := tx.Hash()
	revInfo := &RevocationInfo{RevKey: channel.MyRevocationKeys[hash], TransactionOutput: c, OutputIndex: uint32(index), TransactionHash: hash, ScriptType: scriptType}

	channel.TheirRevocationKeys[hash] = revInfo

	key := channel.MyRevocationKeys[revInfo.TransactionHash]
	rKey := &pro.RevocationKey{Key: key}

	return rKey, nil
}
