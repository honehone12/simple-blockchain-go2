package consensus

import (
	"bytes"
	"encoding/json"
	"log"

	"golang.org/x/exp/slices"
)

func (cs *ConsensusService) handleVote(raw []byte) error {
	if !bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		return nil
	}

	msg := VoteMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !slices.ContainsFunc(cs.nextValidators, func(pk []byte) bool {
		return bytes.Equal(pk, msg.FromPublicKey)
	}) {
		return nil
	}

	cs.aggregate(msg.Ok)
	return nil
}

func (cs *ConsensusService) handleFinalize(raw []byte) error {
	msg := FinalizeMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !bytes.Equal(msg.FromPublicKey, cs.nextAggregator) {
		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	cs.Finalize(msg.Ok)
	return nil
}

func (cs *ConsensusService) handleProposeBlock(raw []byte) error {
	msg := ProposeBlockMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !bytes.Equal(msg.FromPublicKey, cs.nextBlockProducer) {
		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	log.Printf("received block height: %d\n", msg.Block.Info.Height)
	ok, ch, err := cs.producer.Verify(&msg.Block, cs.eCh)
	cs.finalizeCh = ch
	if err != nil {
		return err
	}

	if !ok {
		return cs.vote(ok)
	} else {
		cs.blockCh = cs.syncHandle.NewBlock(&msg.Block)
		return cs.vote(ok)
	}
}

func (cs *ConsensusService) handleStartBlockProcessing(raw []byte) error {
	msg := StartBlockProcessingMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}

	if !(bytes.Equal(msg.NextBlockProducer, cs.nextBlockProducer) &&
		bytes.Equal(msg.NextAggregator, cs.nextAggregator)) {

		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	log.Printf("confirmed comittee, peer: %s", msg.From.Ip4)
	return nil
}

func (cs *ConsensusService) handleGossip(raw []byte) error {
	if cs.syncHandle.IsSyncing() {
		// this node does not go further
		log.Println("syncing...")
		return nil
	}

	msg := GossipMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}

	if msg.IsSyncing {
		// peer is syncing
		return nil
	}
	if len(cs.badPeers) > 0 &&
		slices.ContainsFunc(cs.badPeers, func(pk []byte) bool {
			return bytes.Equal(msg.FromPublicKey, pk)
		}) {

		// peer is bad node
		return nil
	}

	// TODO:
	// add known peer from msg
	// add bad peer from msg

	selfNextHeight := cs.blockchainInfo.NextHeight()
	if msg.NextHeight > selfNextHeight {
		// need sync
		log.Printf("node %d is behind %d\n", selfNextHeight, msg.NextHeight)
		cs.syncHandle.StartSync(msg.NextHeight - 1)
		return nil
	} else if msg.NextHeight < selfNextHeight {
		// this is from new node
		log.Println("received lower height")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}

	if msg.NextEpoch != cs.blockchainInfo.NextEpoch() {
		// this is not expected to happen normally
		log.Println("recieved different epoch")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}
	if msg.NextSlot != cs.blockchainInfo.NextSlot() {
		// this is not expected to happen normally
		log.Println("recieved different slot")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}

	cs.peersStatus[msg.From.Ip4] = msg.IsReady
	cs.addToNextValidators(msg.FromPublicKey)
	return nil
}
