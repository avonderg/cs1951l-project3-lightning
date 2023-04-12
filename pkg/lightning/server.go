package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
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
	return nil, nil
}

func (ln *LightningNode) GetUpdatedTransactions(ctx context.Context, in *pro.TransactionWithAddress) (*pro.UpdatedTransactions, error) {
	// TODO
	return nil, nil
}

func (ln *LightningNode) GetRevocationKey(ctx context.Context, in *pro.SignedTransactionWithKey) (*pro.RevocationKey, error) {
	// TODO
	return nil, nil
}
