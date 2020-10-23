package kumacoin

import (
	"encoding/json"

	"github.com/golang/glog"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
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
	s.ChainConfig.SupportsEstimateSmartFee = false
	s.ChainConfig.SupportsEstimateFee = false

	return s, nil
}

// Initialize initializes KumacoinRPC instance.
func (b *KumacoinRPC) Initialize() error {
	ci, err := b.GetChainInfo()
	if err != nil {
		return err
	}
	chainName := ci.Chain

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

type ResGetBlockInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result bchain.BlockInfo `json:"result"`
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

// GetBlock returns block with given hash.
func (b *KumacoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" && height > 0 {
		hash, err = b.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
	}

	res := ResGetBlockThin{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = false
	err = b.Call(&req, &res)
		if err != nil {
			glog.Errorf("rpc: getblock: block: %s error: %v", hash, err)
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

func IsErrBlockNotFound(err *bchain.RPCError) bool {
	return err.Message == "Block not found" ||
		err.Message == "Block height out of range"
}

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *KumacoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (verbosity=1) ", hash)

	res := ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbose = false
	err := b.Call(&req, &res)

	if err != nil {
		return nil, bchain.ErrBlockNotFound
	}
	if res.Error != nil {
		if IsErrBlockNotFound(res.Error) {
			return nil, bchain.ErrBlockNotFound
		}
		return nil, bchain.ErrBlockNotFound
	}
	return &res.Result, nil
}

// GetTransactionForMempool returns a transaction by the transaction ID
// It could be optimized for mempool, i.e. without block time and confirmations
func (b *KumacoinRPC) GetTransactionForMempool(txid string) (*bchain.Tx, error) {
	return b.GetTransaction(txid)
}

// GetMempoolEntry returns mempool data for given transaction
func (b *KumacoinRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
	return nil, bchain.ErrTxNotFound
}
