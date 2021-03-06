package rawdecodebtc

import (
	"encoding/hex"
	"strings"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// TxRawDecodeResult models the data from the decoderawtransaction command.
type TxRawDecodeResult struct {
	Txid                  string         `json:"txid"`
	Version               int32          `json:"version"`
	Locktime              uint32         `json:"locktime"`
	SerializeSizeStripped int            `json:"sizestripped"`
	SerializeSize         int            `json:"size"`
	Vin                   []btcjson.Vin  `json:"vin"`
	Vout                  []btcjson.Vout `json:"vout"`
}

//FromMessage decodes raw transaction from raw payload
func FromMessage(rawTx []byte, net string) (txReply TxRawDecodeResult, err error) {
	var cparam *chaincfg.Params
	switch net {
	case "regtest":
		cparam = regtest
	case "testnet":
		cparam = testnet
	default:
		cparam = mainnet
	}

	r := strings.NewReader(string(rawTx))
	var mtx wire.MsgTx
	err = mtx.Deserialize(r)
	if err != nil {
		return
	}

	// Create and return the result.
	txReply = TxRawDecodeResult{
		Txid:                  mtx.TxHash().String(),
		Version:               mtx.Version,
		Locktime:              mtx.LockTime,
		SerializeSize:         mtx.SerializeSize(),
		SerializeSizeStripped: mtx.SerializeSizeStripped(),
		Vin:                   CreateVinList(&mtx),
		Vout:                  CreateVoutList(&mtx, cparam, nil),
	}
	return
}

//FromWire decodes wire msg
func FromWire(mtx *wire.MsgTx, net string) (txReply TxRawDecodeResult, err error) {
	var cparam *chaincfg.Params
	switch net {
	case "regtest":
		cparam = regtest
	case "testnet":
		cparam = testnet
	default:
		cparam = mainnet
	}

	// Create and return the result.
	txReply = TxRawDecodeResult{
		Txid:                  mtx.TxHash().String(),
		Version:               mtx.Version,
		Locktime:              mtx.LockTime,
		SerializeSize:         mtx.SerializeSize(),
		SerializeSizeStripped: mtx.SerializeSizeStripped(),
		Vin:                   CreateVinList(mtx),
		Vout:                  CreateVoutList(mtx, cparam, nil),
	}
	return
}

//FromHex decodes raw transaction from Hex payload
func FromHex(message string, net string) (txReply TxRawDecodeResult, err error) {
	hexDecodedTx, err := HexDecodeRawTxString(message)

	var cparam *chaincfg.Params
	switch net {
	case "regtest":
		cparam = regtest
	case "testnet":
		cparam = testnet
	default:
		cparam = mainnet
	}

	r := strings.NewReader(string(hexDecodedTx))
	var mtx wire.MsgTx
	err = mtx.Deserialize(r)
	if err != nil {
		return
	}

	// Create and return the result.
	txReply = TxRawDecodeResult{
		Txid:                  mtx.TxHash().String(),
		Version:               mtx.Version,
		Locktime:              mtx.LockTime,
		SerializeSize:         mtx.SerializeSize(),
		SerializeSizeStripped: mtx.SerializeSizeStripped(),
		Vin:                   CreateVinList(&mtx),
		Vout:                  CreateVoutList(&mtx, cparam, nil),
	}
	return
}

// CreateVinList returns a slice of JSON objects for the inputs of the passed
// transaction.
func CreateVinList(mtx *wire.MsgTx) []btcjson.Vin {
	// Coinbase transactions only have a single txin by definition.
	vinList := make([]btcjson.Vin, len(mtx.TxIn))
	if blockchain.IsCoinBaseTx(mtx) {
		txIn := mtx.TxIn[0]
		vinList[0].Coinbase = hex.EncodeToString(txIn.SignatureScript)
		vinList[0].Sequence = txIn.Sequence
		vinList[0].Witness = witnessToHex(txIn.Witness)
		return vinList
	}

	for i, txIn := range mtx.TxIn {
		// The disassembled string will contain [error] inline
		// if the script doesn't fully parse, so ignore the
		// error here.
		disbuf, _ := txscript.DisasmString(txIn.SignatureScript)

		vinEntry := &vinList[i]
		vinEntry.Txid = txIn.PreviousOutPoint.Hash.String()
		vinEntry.Vout = txIn.PreviousOutPoint.Index
		vinEntry.Sequence = txIn.Sequence
		vinEntry.ScriptSig = &btcjson.ScriptSig{
			Asm: disbuf,
			Hex: hex.EncodeToString(txIn.SignatureScript),
		}

		if mtx.HasWitness() {
			vinEntry.Witness = witnessToHex(txIn.Witness)
		}
	}

	return vinList
}

// CreateVoutList returns a slice of JSON objects for the outputs of the passed
// transaction.
func CreateVoutList(mtx *wire.MsgTx, chainParams *chaincfg.Params, filterAddrMap map[string]struct{}) []btcjson.Vout {
	voutList := make([]btcjson.Vout, 0, len(mtx.TxOut))
	for i, v := range mtx.TxOut {
		// The disassembled string will contain [error] inline if the
		// script doesn't fully parse, so ignore the error here.
		disbuf, _ := txscript.DisasmString(v.PkScript)

		// Ignore the error here since an error means the script
		// couldn't parse and there is no additional information about
		// it anyways.
		scriptClass, addrs, reqSigs, _ := txscript.ExtractPkScriptAddrs(
			v.PkScript, chainParams)

		// Encode the addresses while checking if the address passes the
		// filter when needed.
		passesFilter := len(filterAddrMap) == 0
		encodedAddrs := make([]string, len(addrs))
		for j, addr := range addrs {
			encodedAddr := addr.EncodeAddress()
			encodedAddrs[j] = encodedAddr

			// No need to check the map again if the filter already
			// passes.
			if passesFilter {
				continue
			}
			if _, exists := filterAddrMap[encodedAddr]; exists {
				passesFilter = true
			}
		}

		if !passesFilter {
			continue
		}

		var vout btcjson.Vout
		vout.N = uint32(i)
		vout.Value = btcutil.Amount(v.Value).ToBTC()
		vout.ScriptPubKey.Addresses = encodedAddrs
		vout.ScriptPubKey.Asm = disbuf
		vout.ScriptPubKey.Hex = hex.EncodeToString(v.PkScript)
		vout.ScriptPubKey.Type = scriptClass.String()
		vout.ScriptPubKey.ReqSigs = int32(reqSigs)

		voutList = append(voutList, vout)
	}

	return voutList
}

// witnessToHex formats the passed witness stack as a slice of hex-encoded
// strings to be used in a JSON response.
func witnessToHex(witness wire.TxWitness) []string {
	// Ensure nil is returned when there are no entries versus an empty
	// slice so it can properly be omitted as necessary.
	if len(witness) == 0 {
		return nil
	}

	result := make([]string, 0, len(witness))
	for _, wit := range witness {
		result = append(result, hex.EncodeToString(wit))
	}

	return result
}

// HexDecodeRawTxString hex decodes a rawTx string and returns it as byte slice.
func HexDecodeRawTxString(rawTx string) (hexDecodedTx []byte, err error) {
	hexDecodedTx, err = hex.DecodeString(rawTx)
	if err != nil {
		return
	}
	return
}
