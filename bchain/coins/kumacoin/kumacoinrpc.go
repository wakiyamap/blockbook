package kumacoin

import (
	"math/big"

	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
	"encoding/hex"
	"encoding/json"

	"github.com/golang/glog"
	"github.com/juju/errors"
)

// KumacoinRPC is an interface to JSON-RPC bitcoind service.
type KumacoinRPC struct {
	*btc.BitcoinRPC
}

// NewKumacoinRPC returns new KumacoinRPC instance.
func NewKumacoinRPC(config json.RawMessage, pushHandler func(bchain.NotificationType)) (bchain.BlockChain, error) {
	b, err := btc.NewBitcoinRPC(config, pushHandler)
	if err != nil {
		return nil, err
	}

	s := &KumacoinRPC{
		b.(*btc.BitcoinRPC),
	}
	s.RPCMarshaler = btc.JSONMarshalerV1{}
	s.ChainConfig.SupportsEstimateFee = false

	return s, nil
}

// Initialize initializes KumacoinRPC instance.
func (b *KumacoinRPC) Initialize() error {
	chainName, err := b.GetChainInfoAndInitializeMempool(b)
	if err != nil {
		return err
	}

	glog.Info("Chain name ", chainName)
	params := GetChainParams(chainName)

	// always create parser
	b.Parser = NewKumacoinParser(params, b.ChainConfig)

	// parameters for getInfo request
	if params.Net == MainnetMagic {
		b.Testnet = false
		b.Network = "livenet"
	} else {
		b.Testnet = true
		b.Network = "testnet"
	}

	glog.Info("rpc: block chain ", params.Name)

	return nil
}

// getinfo

type CmdGetInfo struct {
	Method string `json:"method"`
}

type ResGetInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result struct {
		Version         json.Number `json:"version"`
		ProtocolVersion json.Number `json:"protocolversion"`
		Blocks          int32       `json:"blocks"`
		Difficulty      json.Number `json:"difficulty"`
		Testnet         bool        `json:"testnet"`
		Errors          string      `json:"errors"`
	} `json:"result"`
}

// getblockraw

type CmdGetBlockRaw struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
	} `json:"params"`
}

type ResGetBlockRaw struct {
	Error  *bchain.RPCError `json:"error"`
	Result string           `json:"result"`
}

// getblock

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbose   bool   `json:"verbose"`
	} `json:"params"`
}

type BlockThin struct {
	bchain.BlockHeader
	Txids  []string         `json:"tx"`
}

type ResGetBlockThin struct {
	Error  *bchain.RPCError `json:"error"`
	Result BlockThin        `json:"result"`
}

type ResGetBlockFull struct {
	Error  *bchain.RPCError `json:"error"`
	Result bchain.Block     `json:"result"`
}

type ResGetBlockInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result bchain.BlockInfo `json:"result"`
}

// getrawtransaction

type CmdGetRawTransaction struct {
	Method string `json:"method"`
	Params struct {
		Txid    string `json:"txid"`
		Verbose int    `json:"verbose"`
	} `json:"params"`
}

type ResGetRawTransaction struct {
	Error  *bchain.RPCError `json:"error"`
	Result json.RawMessage  `json:"result"`
}


// GetChainInfo returns information about the connected backend
func (b *KumacoinRPC) GetChainInfo() (*bchain.ChainInfo, error) {
	glog.V(1).Info("rpc: getinfo")
	res := ResGetInfo{}
	err := b.Call(&CmdGetInfo{Method: "getinfo"}, &res)
	if err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, res.Error
	}

	bestBlockHash, err := b.GetBlockHash(uint32(res.Result.Blocks))
	if err != nil {
		return nil, err
	}

	var chainType string
	if res.Result.Testnet {
		chainType = "test"
	} else {
		chainType = "main"
	}

	rv := &bchain.ChainInfo{
		Bestblockhash: bestBlockHash,
		Blocks:        int(res.Result.Blocks),
		Chain:         chainType,
		Difficulty:    string(res.Result.Difficulty),
		Headers:       int(res.Result.Blocks),
		SizeOnDisk:    0,
		Subversion:    string(res.Result.Version),
		Timeoffset:    0,
	}
	rv.Version = string(res.Result.Version)
	rv.ProtocolVersion = string(res.Result.ProtocolVersion)
	if len(res.Result.Errors) > 0 {
		rv.Warnings = res.Result.Errors + " "
	}
	if res.Result.Errors != res.Result.Errors {
		rv.Warnings += res.Result.Errors
	}
	return rv, nil
}

func IsErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block height out of range"
}

// GetBlock returns block with given hash.
func (b *KumacoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	res := ResGetBlockThin{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = false
	err := b.Call(&req, &res)
		if err != nil {
			return nil, err
		}
	txs := make([]bchain.Tx, 0, len(res.Result.Txids))
	for _, txid := range res.Result.Txids {
		tx, err := b.GetTransaction(txid)
		if err != nil {
			if err == bchain.ErrTxNotFound {
				glog.Errorf("rpc: getblock: skipping transanction in block %s due error: %s", hash, err)
				continue
			}
			return nil, err
		}
		txs = append(txs, *tx)
	}
	block := &bchain.Block{
		BlockHeader: res.Result.BlockHeader,
		Txs:         txs,
	}
	return block, nil
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *KumacoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (verbosity=false) ", hash)

	res := ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = false
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if IsErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}
	return &res.Result, nil
}

// GetBlockRaw returns block with given hash as bytes
func (b *KumacoinRPC) GetBlockRaw(hash string) ([]byte, error) {
	glog.V(1).Info("rpc: getblockraw", hash)

	res := ResGetBlockRaw{}
	req := CmdGetBlockRaw{Method: "getblockraw"}
	req.Params.BlockHash = hash
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if IsErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}
	return hex.DecodeString(res.Result)
}

// GetBlockFull returns block with given hash
func (b *KumacoinRPC) GetBlockFull(hash string) (*bchain.Block, error) {
	glog.V(1).Info("rpc: getblock (verbosity=true) ", hash)

	res := ResGetBlockFull{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = true
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "hash %v", hash)
	}
	if res.Error != nil {
		if IsErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, errors.Annotatef(res.Error, "hash %v", hash)
	}

	for i := range res.Result.Txs {
		tx := &res.Result.Txs[i]
		for j := range tx.Vout {
			vout := &tx.Vout[j]
			// convert vout.JsonValue to big.Int and clear it, it is only temporary value used for unmarshal
			vout.ValueSat, err = b.Parser.AmountToBigInt(vout.JsonValue)
			if err != nil {
				return nil, err
			}
			vout.JsonValue = ""
		}
	}

	return &res.Result, nil
}

// EstimateSmartFee returns fee estimation
func (b *KumacoinRPC) EstimateSmartFee(_ int, _ bool) (big.Int, error) {
	var r big.Int
	r.SetString("20000", 10)
	return r, nil
}

// EstimateFee returns fee estimation.
func (b *KumacoinRPC) EstimateFee(_ int) (big.Int, error) {
	var r big.Int
	r.SetString("20000", 10)
	return r, nil
}

func IsMissingTx(err *bchain.RPCError) bool {
	if err.Code == -5 { // "No such mempool or blockchain transaction"
		return true
	}
	return false
}

// getRawTransaction returns json as returned by backend, with all coin specific data
func (b *KumacoinRPC) getRawTransaction(txid string) (json.RawMessage, error) {
	glog.V(1).Info("rpc: getrawtransaction ", txid)

	res := ResGetRawTransaction{}
	req := CmdGetRawTransaction{Method: "getrawtransaction"}
	req.Params.Txid = txid
	req.Params.Verbose = 1
	err := b.Call(&req, &res)

	if err != nil {
		return nil, errors.Annotatef(err, "txid %v", txid)
	}
	if res.Error != nil {
		if IsMissingTx(res.Error) {
			return nil, bchain.ErrTxNotFound
		}
		return nil, errors.Annotatef(res.Error, "txid %v", txid)
	}
	return res.Result, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID
// It could be optimized for mempool, i.e. without block time and confirmations
func (b *KumacoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return b.GetTransaction(txid)
}

// GetMempoolEntry returns mempool data for given transaction
func (b *KumacoinRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
	return nil, errors.New("GetMempoolEntry: not implemented")
}
