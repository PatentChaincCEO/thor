// Copyright (c) 2018 The VeChainThor developers

// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying
// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

package xenv

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	"github.com/vechain/thor/abi"
	"github.com/vechain/thor/chain"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
	"github.com/vechain/thor/tx"
	"github.com/vechain/thor/vm/evm"
)

// BlockContext block context.
type BlockContext struct {
	Beneficiary thor.Address
	Signer      thor.Address
	Number      uint32
	Time        uint64
	GasLimit    uint64
	TotalScore  uint64
}

// TransactionContext transaction context.
type TransactionContext struct {
	ID         thor.Bytes32
	Origin     thor.Address
	GasPrice   *big.Int
	ProvedWork *big.Int
	BlockRef   tx.BlockRef
	Expiration uint32
}

// Environment an env to execute native method.
type Environment struct {
	abi      *abi.Method
	seeker   *chain.Seeker
	state    *state.State
	blockCtx *BlockContext
	txCtx    *TransactionContext
	evm      *evm.EVM
	contract *evm.Contract
}

// New create a new env.
func New(
	abi *abi.Method,
	seeker *chain.Seeker,
	state *state.State,
	blockCtx *BlockContext,
	txCtx *TransactionContext,
	evm *evm.EVM,
	contract *evm.Contract,
) *Environment {
	return &Environment{
		abi:      abi,
		seeker:   seeker,
		state:    state,
		blockCtx: blockCtx,
		txCtx:    txCtx,
		evm:      evm,
		contract: contract,
	}
}

func (env *Environment) Seeker() *chain.Seeker                   { return env.seeker }
func (env *Environment) State() *state.State                     { return env.state }
func (env *Environment) TransactionContext() *TransactionContext { return env.txCtx }
func (env *Environment) BlockContext() *BlockContext             { return env.blockCtx }
func (env *Environment) Caller() thor.Address                    { return thor.Address(env.contract.Caller()) }
func (env *Environment) To() thor.Address                        { return thor.Address(env.contract.Address()) }

func (env *Environment) UseGas(gas uint64) {
	if !env.contract.UseGas(gas) {
		panic(evm.ErrOutOfGas)
	}
}

func (env *Environment) ParseArgs(val interface{}) {
	if err := env.abi.DecodeInput(env.contract.Input, val); err != nil {
		// as vm error
		panic(errors.WithMessage(err, "decode native input"))
	}
}

func (env *Environment) Must(cond bool) {
	if !cond {
		panic("false condition")
	}
}

func (env *Environment) Log(abi *abi.Event, address thor.Address, topics []thor.Bytes32, args ...interface{}) {
	data, err := abi.Encode(args...)
	if err != nil {
		panic(errors.WithMessage(err, "encode native event"))
	}
	env.UseGas(ethparams.LogGas + ethparams.LogTopicGas*uint64(len(topics)) + ethparams.LogDataGas*uint64(len(data)))

	ethTopics := make([]common.Hash, 0, len(topics)+1)
	ethTopics = append(ethTopics, common.Hash(abi.ID()))
	for _, t := range topics {
		ethTopics = append(ethTopics, common.Hash(t))
	}
	env.evm.StateDB.AddLog(&types.Log{
		Address: common.Address(address),
		Topics:  ethTopics,
		Data:    data,
	})
}

func (env *Environment) Call(proc func(env *Environment) []interface{}) (output []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			if e == evm.ErrOutOfGas {
				err = evm.ErrOutOfGas
			} else {
				panic(e)
			}
		}
	}()
	data, err := env.abi.EncodeOutput(proc(env)...)
	if err != nil {
		panic(errors.WithMessage(err, "encode native output"))
	}
	return data, nil
}
