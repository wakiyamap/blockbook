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
		Testnet         bool        `json:"testnet"`
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

// getblockcount

type CmdGetBlockCount struct {
	Method string `json:"method"`
}

type ResGetBlockCount struct {
	Error  *bchain.RPCError `json:"error"`
	Result uint32           `json:"result"`
}

// getblock

type CmdGetBlock struct {
	Method string `json:"method"`
	Params struct {
		BlockHash string `json:"blockhash"`
		Verbosity string `json:"verbosity"`
	} `json:"params"`
}

type ResGetBlockRaw struct {
	Error  *bchain.RPCError `json:"error"`
	Result string           `json:"result"`
}

type ResGetBlockFull struct {
	Error  *bchain.RPCError `json:"error"`
	Result bchain.Block     `json:"result"`
}

type ResGetBlockInfo struct {
	Error  *bchain.RPCError `json:"error"`
	Result bchain.BlockInfo `json:"result"`
}

// GetBestBlockHash returns hash of the tip of the best-block-chain.
func (b *KumacoinRPC) GetBestBlockHash() (string, error) {
	glog.V(1).Info("rpc: getblockcount")

	res := ResGetBlockCount{}
	req := CmdGetBlockCount{Method: "getblockcount"}
	err := b.Call(&req, &res)

	if err != nil {
		return "", err
	}
	if res.Error != nil {
		return "", res.Error
	}

	hash, err := b.GetBlockHash(res.Result)
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

	bestBlockHash, err := b.GetBestBlockHash()
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

// GetBlockInfo returns extended header (more info than in bchain.BlockHeader) with a list of txids
func (b *BitcoinRPC) GetBlockInfo(hash string) (*bchain.BlockInfo, error) {
	glog.V(1).Info("rpc: getblock (verbosity= ) ", hash)

	res := ResGetBlockInfo{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	// req.Params.Verbosity = ""
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
func (b *BitcoinRPC) GetBlockRaw(hash string) ([]byte, error) {
	glog.V(1).Info("rpc: getblock (verbosity=false) ", hash)

	res := ResGetBlockRaw{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = false
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
func (b *BitcoinRPC) GetBlockFull(hash string) (*bchain.Block, error) {
	glog.V(1).Info("rpc: getblock (verbosity=true) ", hash)

	res := ResGetBlockFull{}
	req := CmdGetBlock{Method: "getblock"}
	req.Params.BlockHash = hash
	req.Params.Verbosity = true
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

// GetMempoolEntry returns mempool data for given transaction
func (b *KumacoinRPC) GetMempoolEntry(txid string) (*bchain.MempoolEntry, error) {
	return nil, errors.New("GetMempoolEntry: not implemented")
}
