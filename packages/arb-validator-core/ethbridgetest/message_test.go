/*
 * Copyright 2019, Offchain Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ethbridgetest

import (
	"context"
	"errors"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/offchainlabs/arbitrum/packages/arb-util/common"
	"github.com/offchainlabs/arbitrum/packages/arb-util/hashing"
	"github.com/offchainlabs/arbitrum/packages/arb-util/value"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/ethbridge"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/ethbridge/messagetester"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/message"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/test"
)

var privHex = "27e926925fb5903ee038c894d9880f74d3dd6518e23ab5e5651de93327c7dffa"

var auth *bind.TransactOpts
var tester *messagetester.MessageTester
var client *ethclient.Client

var addr1 common.Address
var addr2 common.Address
var addr3 common.Address

var errHash = errors.New("ethbridge calculated wrong hash")
var errMsgHash = errors.New("ethbridge calculated wrong message hash")

func TestMain(m *testing.M) {
	addr1[0] = 76
	addr1[19] = 93
	addr2[0] = 43
	addr2[19] = 12
	addr3[0] = 73
	addr3[19] = 85

	var err error

	auth, err = test.SetupAuth(privHex)
	if err != nil {
		log.Fatal(err)
	}
	client, err = ethclient.Dial(test.GetEthUrl())
	if err != nil {
		log.Fatal(err)
	}
	_, tx, deployedTester, err := messagetester.DeployMessageTester(
		auth,
		client,
	)
	if err != nil {
		log.Fatal(err)
	}
	_, err = ethbridge.WaitForReceiptWithResults(
		context.Background(),
		client,
		auth.From,
		tx,
		"DeployMessageTester",
	)
	if err != nil {
		log.Fatal(err)
	}
	tester = deployedTester
	code := m.Run()
	os.Exit(code)
}

func TestTransactionMessage(t *testing.T) {
	msg := message.DeliveredTransaction{
		Transaction: message.Transaction{
			Chain:       addr3,
			To:          addr1,
			From:        addr2,
			SequenceNum: big.NewInt(74563),
			Value:       big.NewInt(89735406),
			Data:        []byte{65, 23, 68, 87, 12},
		},
		BlockNum:  common.NewTimeBlocks(big.NewInt(87962345)),
		Timestamp: big.NewInt(35463245),
	}
	bridgeHash, err := tester.TransactionHash(
		nil,
		msg.Chain.ToEthAddress(),
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.SequenceNum,
		msg.Value,
		msg.Data,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHash != msg.CommitmentHash().ToEthHash() {
		t.Error(errHash)
	}

	messageBridgeHash, err := tester.TransactionMessageHash(
		nil,
		msg.Chain.ToEthAddress(),
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.SequenceNum,
		msg.Value,
		msg.Data,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}

	if messageBridgeHash != message.DeliveredValue(msg).Hash().ToEthHash() {
		t.Error(errMsgHash)
	}
}

func TestTransactionBatchMessage(t *testing.T) {
	chain := addr3
	txData := []byte{65, 23, 68, 87, 12}
	tos := []common.Address{addr1}
	sequenceNums := []*big.Int{big.NewInt(74563)}
	values := []*big.Int{big.NewInt(89735406)}

	offchainHash := message.BatchTxHash(
		chain,
		tos[0],
		sequenceNums[0],
		values[0],
		txData,
	)
	messageHash := hashing.SoliditySHA3WithPrefix(offchainHash[:])

	privateKey, err := crypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}

	sigBytes, err := crypto.Sign(messageHash.Bytes(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var sig [65]byte
	copy(sig[:], sigBytes)

	data := message.BatchTxData(
		tos[0],
		sequenceNums[0],
		values[0],
		txData,
		sig,
	)

	msg := message.DeliveredTransactionBatch{
		TransactionBatch: message.TransactionBatch{
			Chain:  addr3,
			TxData: data,
		},
		BlockNum:  common.NewTimeBlocks(big.NewInt(87962345)),
		Timestamp: big.NewInt(35463245),
	}

	bridgeHash, err := tester.TransactionBatchHash(
		nil,
		data,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHash != msg.CommitmentHash().ToEthHash() {
		t.Error(errHash)
	}

	bridgeInboxHash, err := tester.TransactionMessageBatchHash(
		nil,
		value.NewEmptyTuple().Hash(),
		msg.Chain.ToEthAddress(),
		data,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	valInboxHash := message.AddToPrev(value.NewEmptyTuple(), msg).Hash()
	if bridgeInboxHash != valInboxHash.ToEthHash() {
		t.Error(errMsgHash)
	}
}

func TestEthMessage(t *testing.T) {
	msg := message.DeliveredEth{
		Eth: message.Eth{
			To:    addr1,
			From:  addr2,
			Value: big.NewInt(89735406),
		},
		BlockNum:   common.NewTimeBlocks(big.NewInt(87962345)),
		Timestamp:  big.NewInt(35463245),
		MessageNum: big.NewInt(98742),
	}
	bridgeHash, err := tester.EthHash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.Value,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHash != msg.CommitmentHash().ToEthHash() {
		t.Error(errHash)
	}

	messageBridgeHash, err := tester.EthMessageHash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.Value,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}

	if messageBridgeHash != message.DeliveredValue(msg).Hash().ToEthHash() {
		t.Error(errMsgHash)
	}
}

func TestERC20Message(t *testing.T) {
	msg := message.DeliveredERC20{
		ERC20: message.ERC20{
			To:           addr1,
			From:         addr2,
			TokenAddress: addr3,
			Value:        big.NewInt(89735406),
		},
		BlockNum:   common.NewTimeBlocks(big.NewInt(87962345)),
		Timestamp:  big.NewInt(35463245),
		MessageNum: big.NewInt(98742),
	}
	bridgeHash, err := tester.Erc20Hash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.TokenAddress.ToEthAddress(),
		msg.Value,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHash != msg.CommitmentHash().ToEthHash() {
		t.Error(errHash)
	}

	messageBridgeHash, err := tester.Erc20MessageHash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.TokenAddress.ToEthAddress(),
		msg.Value,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}

	if messageBridgeHash != message.DeliveredValue(msg).Hash().ToEthHash() {
		t.Error(errMsgHash)
	}
}

func TestERC721Message(t *testing.T) {
	msg := message.DeliveredERC721{
		ERC721: message.ERC721{
			To:           addr1,
			From:         addr2,
			TokenAddress: addr3,
			Id:           big.NewInt(89735406),
		},
		BlockNum:   common.NewTimeBlocks(big.NewInt(87962345)),
		Timestamp:  big.NewInt(35463245),
		MessageNum: big.NewInt(98742),
	}
	bridgeHash, err := tester.Erc721Hash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.TokenAddress.ToEthAddress(),
		msg.Id,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}
	if bridgeHash != msg.CommitmentHash().ToEthHash() {
		t.Error(errHash)
	}

	messageBridgeHash, err := tester.Erc721MessageHash(
		nil,
		msg.To.ToEthAddress(),
		msg.From.ToEthAddress(),
		msg.TokenAddress.ToEthAddress(),
		msg.Id,
		msg.BlockNum.AsInt(),
		msg.Timestamp,
		msg.MessageNum,
	)
	if err != nil {
		t.Fatal(err)
	}

	if messageBridgeHash != message.DeliveredValue(msg).Hash().ToEthHash() {
		t.Error(errMsgHash)
	}
}
