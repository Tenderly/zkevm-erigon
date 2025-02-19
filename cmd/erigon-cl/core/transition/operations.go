package transition

import (
	"errors"
	"fmt"

	"github.com/Giulio2002/bls"
	"github.com/ledgerwatch/log/v3"

	"github.com/tenderly/zkevm-erigon/cl/clparams"
	"github.com/tenderly/zkevm-erigon/cl/cltypes"
	"github.com/tenderly/zkevm-erigon/cl/fork"
	"github.com/tenderly/zkevm-erigon/cl/utils"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/state"
	"github.com/tenderly/zkevm-erigon/core/types"
)

func ProcessProposerSlashing(state *state.BeaconState, propSlashing *cltypes.ProposerSlashing) error {
	h1 := propSlashing.Header1.Header
	h2 := propSlashing.Header2.Header

	if h1.Slot != h2.Slot {
		return fmt.Errorf("non-matching slots on proposer slashing: %d != %d", h1.Slot, h2.Slot)
	}

	if h1.ProposerIndex != h2.ProposerIndex {
		return fmt.Errorf("non-matching proposer indices proposer slashing: %d != %d", h1.ProposerIndex, h2.ProposerIndex)
	}

	h1Root, err := h1.HashSSZ()
	if err != nil {
		return fmt.Errorf("unable to hash header1: %v", err)
	}
	h2Root, err := h2.HashSSZ()
	if err != nil {
		return fmt.Errorf("unable to hash header2: %v", err)
	}
	if h1Root == h2Root {
		return fmt.Errorf("propose slashing headers are the same: %v == %v", h1Root, h2Root)
	}

	proposer, err := state.ValidatorForValidatorIndex(int(h1.ProposerIndex))
	if err != nil {
		return err
	}
	if !proposer.IsSlashable(state.Epoch()) {
		return fmt.Errorf("proposer is not slashable: %v", proposer)
	}

	for _, signedHeader := range []*cltypes.SignedBeaconBlockHeader{propSlashing.Header1, propSlashing.Header2} {
		domain, err := state.GetDomain(state.BeaconConfig().DomainBeaconProposer, state.GetEpochAtSlot(signedHeader.Header.Slot))
		if err != nil {
			return fmt.Errorf("unable to get domain: %v", err)
		}
		signingRoot, err := fork.ComputeSigningRoot(signedHeader.Header, domain)
		if err != nil {
			return fmt.Errorf("unable to compute signing root: %v", err)
		}
		valid, err := bls.Verify(signedHeader.Signature[:], signingRoot[:], proposer.PublicKey[:])
		if err != nil {
			return fmt.Errorf("unable to verify signature: %v", err)
		}
		if !valid {
			return fmt.Errorf("invalid signature: signature %v, root %v, pubkey %v", signedHeader.Signature[:], signingRoot[:], proposer.PublicKey[:])
		}
	}

	// Set whistleblower index to 0 so current proposer gets reward.
	state.SlashValidator(h1.ProposerIndex, nil)
	return nil
}

func ProcessAttesterSlashing(state *state.BeaconState, attSlashing *cltypes.AttesterSlashing) error {
	att1 := attSlashing.Attestation_1
	att2 := attSlashing.Attestation_2

	if !cltypes.IsSlashableAttestationData(att1.Data, att2.Data) {
		return fmt.Errorf("attestation data not slashable: %+v; %+v", att1.Data, att2.Data)
	}

	valid, err := state.IsValidIndexedAttestation(att1)
	if err != nil {
		return fmt.Errorf("error calculating indexed attestation 1 validity: %v", err)
	}
	if !valid {
		return fmt.Errorf("invalid indexed attestation 1")
	}

	valid, err = state.IsValidIndexedAttestation(att2)
	if err != nil {
		return fmt.Errorf("error calculating indexed attestation 2 validity: %v", err)
	}
	if !valid {
		return fmt.Errorf("invalid indexed attestation 2")
	}

	slashedAny := false
	currentEpoch := state.GetEpochAtSlot(state.Slot())
	for _, ind := range utils.IntersectionOfSortedSets(att1.AttestingIndices, att2.AttestingIndices) {
		validator, err := state.ValidatorForValidatorIndex(int(ind))
		if err != nil {
			return err
		}
		if validator.IsSlashable(currentEpoch) {
			err := state.SlashValidator(ind, nil)
			if err != nil {
				return fmt.Errorf("unable to slash validator: %d", ind)
			}
			slashedAny = true
		}
	}

	if !slashedAny {
		return fmt.Errorf("no validators slashed")
	}
	return nil
}

func ProcessDeposit(state *state.BeaconState, deposit *cltypes.Deposit, fullValidation bool) error {
	if deposit == nil {
		return nil
	}
	depositLeaf, err := deposit.Data.HashSSZ()
	if err != nil {
		return err
	}
	depositIndex := state.Eth1DepositIndex()
	eth1Data := state.Eth1Data()
	// Validate merkle proof for deposit leaf.
	if fullValidation && !utils.IsValidMerkleBranch(
		depositLeaf,
		deposit.Proof,
		state.BeaconConfig().DepositContractTreeDepth+1,
		depositIndex,
		eth1Data.Root,
	) {
		return fmt.Errorf("processDepositForAltair: Could not validate deposit root")
	}

	// Increment index
	state.SetEth1DepositIndex(depositIndex + 1)
	publicKey := deposit.Data.PubKey
	amount := deposit.Data.Amount
	// Check if pub key is in validator set
	validatorIndex, has := state.ValidatorIndexByPubkey(publicKey)
	if !has {
		// Agnostic domain.
		domain, err := fork.ComputeDomain(state.BeaconConfig().DomainDeposit[:], utils.Uint32ToBytes4(state.BeaconConfig().GenesisForkVersion), [32]byte{})
		if err != nil {
			return err
		}
		depositMessageRoot, err := deposit.Data.MessageHash()
		if err != nil {
			return err
		}
		signedRoot := utils.Keccak256(depositMessageRoot[:], domain)
		// Perform BLS verification and if successful noice.
		valid, err := bls.Verify(deposit.Data.Signature[:], signedRoot[:], publicKey[:])
		// Literally you can input it trash.
		if !valid || err != nil {
			log.Debug("Validator BLS verification failed", "valid", valid, "err", err)
			return nil
		}
		// Append validator
		state.AddValidator(state.ValidatorFromDeposit(deposit), amount)
		// Altair forward
		if state.Version() >= clparams.AltairVersion {
			state.AddCurrentEpochParticipationFlags(cltypes.ParticipationFlags(0))
			state.AddPreviousEpochParticipationFlags(cltypes.ParticipationFlags(0))
			state.AddInactivityScore(0)
		}
		return nil
	}
	// Increase the balance if exists already
	return state.IncreaseBalance(validatorIndex, amount)
}

// ProcessVoluntaryExit takes a voluntary exit and applies state transition.
func ProcessVoluntaryExit(state *state.BeaconState, signedVoluntaryExit *cltypes.SignedVoluntaryExit, fullValidation bool) error {
	// Sanity checks so that we know it is good.
	voluntaryExit := signedVoluntaryExit.VolunaryExit
	currentEpoch := state.Epoch()
	validator, err := state.ValidatorForValidatorIndex(int(voluntaryExit.ValidatorIndex))
	if err != nil {
		return err
	}
	if !validator.Active(currentEpoch) {
		return errors.New("ProcessVoluntaryExit: validator is not active")
	}
	if validator.ExitEpoch != state.BeaconConfig().FarFutureEpoch {
		return errors.New("ProcessVoluntaryExit: another exit for the same validator is already getting processed")
	}
	if currentEpoch < voluntaryExit.Epoch {
		return errors.New("ProcessVoluntaryExit: exit is happening in the future")
	}
	if currentEpoch < validator.ActivationEpoch+state.BeaconConfig().ShardCommitteePeriod {
		return errors.New("ProcessVoluntaryExit: exit is happening too fast")
	}

	// We can skip it in some instances if we want to optimistically sync up.
	if fullValidation {
		domain, err := state.GetDomain(state.BeaconConfig().DomainVoluntaryExit, voluntaryExit.Epoch)
		if err != nil {
			return err
		}
		signingRoot, err := fork.ComputeSigningRoot(voluntaryExit, domain)
		if err != nil {
			return err
		}
		valid, err := bls.Verify(signedVoluntaryExit.Signature[:], signingRoot[:], validator.PublicKey[:])
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("ProcessVoluntaryExit: BLS verification failed")
		}
	}
	// Do the exit (same process in slashing).
	return state.InitiateValidatorExit(voluntaryExit.ValidatorIndex)
}

// ProcessWithdrawals processes withdrawals by decreasing the balance of each validator
// and updating the next withdrawal index and validator index.
func ProcessWithdrawals(state *state.BeaconState, withdrawals types.Withdrawals, fullValidation bool) error {
	// Get the list of withdrawals, the expected withdrawals (if performing full validation),
	// and the beacon configuration.
	beaconConfig := state.BeaconConfig()
	numValidators := uint64(len(state.Validators()))

	// Check if full validation is required and verify expected withdrawals.
	if fullValidation {
		expectedWithdrawals := state.ExpectedWithdrawals()
		if len(expectedWithdrawals) != len(withdrawals) {
			return fmt.Errorf("ProcessWithdrawals: expected %d withdrawals, but got %d", len(expectedWithdrawals), len(withdrawals))
		}
		for i, withdrawal := range withdrawals {
			if !expectedWithdrawals[i].Equal(withdrawal) {
				return fmt.Errorf("ProcessWithdrawals: withdrawal %d does not match expected withdrawal", i)
			}
		}
	}

	// Decrease the balance of each validator for the corresponding withdrawal.
	for _, withdrawal := range withdrawals {
		if err := state.DecreaseBalance(withdrawal.Validator, withdrawal.Amount); err != nil {
			return err
		}
	}

	// Update next withdrawal index based on number of withdrawals.
	if len(withdrawals) > 0 {
		lastWithdrawalIndex := withdrawals[len(withdrawals)-1].Index
		state.SetNextWithdrawalIndex(lastWithdrawalIndex + 1)
	}

	// Update next withdrawal validator index based on number of withdrawals.
	if len(withdrawals) == int(beaconConfig.MaxWithdrawalsPerPayload) {
		lastWithdrawalValidatorIndex := withdrawals[len(withdrawals)-1].Validator + 1
		state.SetNextWithdrawalValidatorIndex(lastWithdrawalValidatorIndex % numValidators)
	} else {
		nextIndex := state.NextWithdrawalValidatorIndex() + beaconConfig.MaxValidatorsPerWithdrawalsSweep
		state.SetNextWithdrawalValidatorIndex(nextIndex % numValidators)
	}

	return nil
}
