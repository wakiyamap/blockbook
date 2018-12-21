package kumacoin

import (
	"blockbook/bchain/coins/btc"

	"github.com/btcsuite/btcd/wire"
	"github.com/jakm/btcutil/chaincfg"
)

const (
	MainnetMagic wire.BitcoinNet = 0xfed2d4c3
	TestnetMagic wire.BitcoinNet = 0xefc0f2cd
)

var (
	MainNetParams chaincfg.Params
	TestNetParams chaincfg.Params
)

func init() {
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Net = MainnetMagic
	MainNetParams.PubKeyHashAddrID = []byte{45}
	MainNetParams.ScriptHashAddrID = []byte{8}
	MainNetParams.Bech32HRPSegwit = "kuma"

	TestNetParams = chaincfg.TestNet3Params
	TestNetParams.Net = TestnetMagic
	TestNetParams.PubKeyHashAddrID = []byte{117}
	TestNetParams.ScriptHashAddrID = []byte{196}
	TestNetParams.Bech32HRPSegwit = "tkuma"
}

// KumacoinParser handle
type KumacoinParser struct {
	*btc.BitcoinParser
}

// NewKumacoinParser returns new KumacoinParser instance
func NewKumacoinParser(params *chaincfg.Params, c *btc.Configuration) *KumacoinParser {
	return &KumacoinParser{BitcoinParser: btc.NewBitcoinParser(params, c)}
}

// GetChainParams contains network parameters for the main Kumacoin network,
// and the test Kumacoin network
func GetChainParams(chain string) *chaincfg.Params {
	// register bitcoin parameters in addition to Kumacoin parameters
	// kumacoin has dual standard of addresses and we want to be able to
	// parse both standards
	if !chaincfg.IsRegistered(&chaincfg.MainNetParams) {
		chaincfg.RegisterBitcoinParams()
	}
	if !chaincfg.IsRegistered(&MainNetParams) {
		err := chaincfg.Register(&MainNetParams)
		if err == nil {
			err = chaincfg.Register(&TestNetParams)
		}
		if err != nil {
			panic(err)
		}
	}
	switch chain {
	case "test":
		return &TestNetParams
	default:
		return &MainNetParams
	}
}
