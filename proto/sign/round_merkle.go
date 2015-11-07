package sign
import (
	"github.com/dedis/cothority/lib/hashid"
	"sort"
	"github.com/dedis/cothority/lib/proof"
	"bytes"
	dbg "github.com/dedis/cothority/lib/debug_lvl"
	"github.com/dedis/crypto/abstract"
	"errors"
"github.com/dedis/cothority/lib/coconet"
)

/*
 * This is a module for the round-struct that does all the
 * calculation for a merkle-hash-tree.
 */

// Create round lasting secret and commit point v and V
// Initialize log structure for the round
func (round *Round) InitCommitCrypto() {
	// generate secret and point commitment for this round
	rand := round.Suite.Cipher([]byte(round.Name))
	round.Log = SNLog{}
	round.Log.v = round.Suite.Secret().Pick(rand)
	round.Log.V = round.Suite.Point().Mul(nil, round.Log.v)
	// initialize product of point commitments
	round.Log.V_hat = round.Suite.Point().Null()
	round.Log.Suite = round.Suite
	round.Add(round.Log.V_hat, round.Log.V)

	round.X_hat = round.Suite.Point().Null()
	round.Add(round.X_hat, round.PubKey)
}

// Adds a child-node to the Merkle-tree and updates the root-hashes
func (round *Round) MerkleAddChildren() {
	// children commit roots
	round.CMTRoots = make([]hashid.HashId, len(round.Leaves))
	copy(round.CMTRoots, round.Leaves)
	round.CMTRootNames = make([]string, len(round.Leaves))
	copy(round.CMTRootNames, round.LeavesFrom)

	// concatenate children commit roots in one binary blob for easy marshalling
	round.Log.CMTRoots = make([]byte, 0)
	for _, leaf := range round.Leaves {
		round.Log.CMTRoots = append(round.Log.CMTRoots, leaf...)
	}
}

// Adds the local Merkle-tree root, usually from a stamper or
// such
func (round *Round) MerkleAddLocal(localMTroot hashid.HashId) {
	// add own local mtroot to leaves
	round.LocalMTRoot = localMTroot
	round.Leaves = append(round.Leaves, round.LocalMTRoot)
}

// Hashes the log of the round-structure
func (round *Round) MerkleHashLog() error {
	var err error

	h := round.Suite.Hash()
	logBytes, err := round.Log.MarshalBinary()
	if err != nil {
		return err
	}
	h.Write(logBytes)
	round.HashedLog = h.Sum(nil)
	return err
}


func (round *Round) ComputeCombinedMerkleRoot() {
	// add hash of whole log to leaves
	round.Leaves = append(round.Leaves, round.HashedLog)

	// compute MT root based on Log as right child and
	// MT of leaves as left child and send it up to parent
	sort.Sort(hashid.ByHashId(round.Leaves))
	left, proofs := proof.ProofTree(round.Suite.Hash, round.Leaves)
	right := round.HashedLog
	moreLeaves := make([]hashid.HashId, 0)
	moreLeaves = append(moreLeaves, left, right)
	round.MTRoot, _ = proof.ProofTree(round.Suite.Hash, moreLeaves)

	// Hashed Log has to come first in the proof; len(sn.CMTRoots)+1 proofs
	round.Proofs = make(map[string]proof.Proof, 0)
	for name := range round.Children {
		round.Proofs[name] = append(round.Proofs[name], right)
	}
	round.Proofs["local"] = append(round.Proofs["local"], right)

	// separate proofs by children (need to send personalized proofs to children)
	// also separate local proof (need to send it to timestamp server)
	round.SeparateProofs(proofs, round.Leaves)
}

// Identify which proof corresponds to which leaf
// Needed given that the leaves are sorted before passed to the function that create
// the Merkle Tree and its Proofs
func (round *Round) SeparateProofs(proofs []proof.Proof, leaves []hashid.HashId) {
	// separate proofs for children servers mt roots
	for i := 0; i < len(round.CMTRoots); i++ {
		name := round.CMTRootNames[i]
		for j := 0; j < len(leaves); j++ {
			if bytes.Compare(round.CMTRoots[i], leaves[j]) == 0 {
				// sn.Proofs[i] = append(sn.Proofs[i], proofs[j]...)
				round.Proofs[name] = append(round.Proofs[name], proofs[j]...)
				continue
			}
		}
	}

	// separate proof for local mt root
	for j := 0; j < len(leaves); j++ {
		if bytes.Compare(round.LocalMTRoot, leaves[j]) == 0 {
			round.Proofs["local"] = append(round.Proofs["local"], proofs[j]...)
		}
	}
}

func (round *Round) InitResponseCrypto() {
	round.R = round.Suite.Secret()
	round.R.Mul(round.PrivKey, round.C).Sub(round.Log.v, round.R)
	// initialize sum of children's responses
	round.R_hat = round.R
}

// Create Merkle Proof for local client (timestamp server) and
// store it in Node so that we can send it to the clients during
// the SignatureBroadcast
func (round *Round) StoreLocalMerkleProof(chm *ChallengeMessage) error {
	proofForClient := make(proof.Proof, len(chm.Proof))
	copy(proofForClient, chm.Proof)

	// To the proof from our root to big root we must add the separated proof
	// from the localMKT of the client (timestamp server) to our root
	proofForClient = append(proofForClient, round.Proofs["local"]...)

	// if want to verify partial and full proofs
	if dbg.DebugVisible > 2 {
		//sn.VerifyAllProofs(view, chm, proofForClient)
	}
	round.Proof = proofForClient
	round.MTRoot = chm.MTRoot
	return nil
}

// Figure out which kids did not submit messages
// Add default messages to messgs, one per missing child
// as to make it easier to identify and add them to exception lists in one place
func (round *Round) FillInWithDefaultMessages() []*SigningMessage {
	children := round.Children

	messgs := round.Responses
	allmessgs := make([]*SigningMessage, len(messgs))
	copy(allmessgs, messgs)

	for c := range children {
		found := false
		for _, m := range messgs {
			if m.From == c {
				found = true
				break
			}
		}

		if !found {
			allmessgs = append(allmessgs, &SigningMessage{View: round.View,
				Type: Default, From: c})
		}
	}

	return allmessgs
}

// Called by every node after receiving aggregate responses from descendants
func (round *Round) VerifyResponses() error {

	// Check that: base**r_hat * X_hat**c == V_hat
	// Equivalent to base**(r+xc) == base**(v) == T in vanillaElGamal
	Aux := round.Suite.Point()
	V_clean := round.Suite.Point()
	V_clean.Add(V_clean.Mul(nil, round.R_hat), Aux.Mul(round.X_hat, round.C))
	// T is the recreated V_hat
	T := round.Suite.Point().Null()
	T.Add(T, V_clean)
	T.Add(T, round.ExceptionV_hat)

	var c2 abstract.Secret
	isroot := round.Parent == ""
	if isroot {
		// round challenge must be recomputed given potential
		// exception list
		msg := round.Msg
		msg = append(msg, []byte(round.MTRoot)...)
		round.C = HashElGamal(round.Suite, msg, round.Log.V_hat)
		c2 = HashElGamal(round.Suite, msg, T)
	}

	// intermediary nodes check partial responses aginst their partial keys
	// the root node is also able to check against the challenge it emitted
	if !T.Equal(round.Log.V_hat) || (isroot && !round.C.Equal(c2)) {
		return errors.New("Verifying ElGamal Collective Signature failed in " +
		round.Name)
	} else if isroot {
		dbg.Lvl4(round.Name, "reports ElGamal Collective Signature succeeded")
	}
	return nil
}

// Create Personalized Merkle Proofs for children servers
// Send Personalized Merkle Proofs to children servers
func (round *Round) SendChildrenChallengesProofs(chm *ChallengeMessage) error {
	// proof from big root to our root will be sent to all children
	baseProof := make(proof.Proof, len(chm.Proof))
	copy(baseProof, chm.Proof)

	// for each child, create personalized part of proof
	// embed it in SigningMessage, and send it
	for name, conn := range round.Children {
		newChm := *chm
		newChm.Proof = append(baseProof, round.Proofs[name]...)

		var messg coconet.BinaryMarshaler
		messg = &SigningMessage{View: round.View, Type: Challenge, Chm: &newChm}

		// send challenge message to child
		// dbg.Lvl4("connection: sending children challenge proofs:", name, conn)
		if err := conn.PutData(messg); err != nil {
			return err
		}
	}

	return nil
}

// Send children challenges
func (round *Round) SendChildrenChallenges(chm *ChallengeMessage) error {
	for _, child := range round.Children {
		var messg coconet.BinaryMarshaler
		messg = &SigningMessage{View: round.View, Type: Challenge, Chm: chm}

		// fmt.Println(sn.Name(), "send to", i, child, "on view", view)
		if err := child.PutData(messg); err != nil {
			return err
		}
	}

	return nil
}
