package kumacoin

import (
	"math/big"

	"blockbook/bchain"
	"blockbook/bchain/coins/btc"
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
	s.RPCMarshaler = btc.JSONMarshalerV2{}
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
		Testnet         string      `json:"testnet"`
		Errors          string      `json:"errors"`
	} `json:"result"`
}

// getblockheader

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbose   bool   `json:"verbose"`
	} `json:"params"`
}

type ResGetBlock struct {
	Error  *bchain.RPCError   `json:"error"`
	Result bchain.BlockHeader `json:"result"`
}

// GetBestBlockHash returns hash of the tip of the best-block-chain.
func (b *KumacoinRPC) GetBestBlockHash() (string, error) {
	glog.V(1).Info("rpc: getinfo")
	res := ResGetInfo{}
	err := b.Call(&CmdGetInfo{Method: "getinfo"}, &res)
	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}

	hash, err := b.GetBlockHash(uint32(res.Result.Blocks))
	if err != nil {
		return "", err
	}
	return hash, nil

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

	hash, err := b.GetBlockHash(uint32(res.Result.Blocks))
	if err != nil {
		return nil, err
	}
	
	rv := &bchain.ChainInfo{
		Bestblockhash: hash,
		Blocks:        int(res.Result.Blocks),
		Chain:         res.Result.Testnet,
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

// GetBlockHeader returns header of block with given hash.
func (b *KumacoinRPC) GetBlockHeader(hash string) (*bchain.BlockHeader, error) {
	glog.V(1).Info("rpc: getblock")

	res := ResGetBlock{}
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
	return &res.Result, nil
}

// GetBlock returns block with given hash.
func (b *KumacoinRPC) GetBlock(hash string, height uint32) (*bchain.Block, error) {
	var err error
	if hash == "" {
		hash, err = b.GetBlockHash(height)
		if err != nil {
			return nil, err
		}
	}
	if !b.ParseBlocks {
		return b.GetBlockFull(hash)
	}
	return b.GetBlockWithoutHeader(hash, height)
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

// GetMempoolEntry returns mempool data for given transaction
func (b *KumacoinRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
	return nil, errors.New("GetMempoolEntry: not implemented")
}
