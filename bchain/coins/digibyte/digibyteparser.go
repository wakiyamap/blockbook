package digibyte

import (
	"blockbook/bchain/coins/btc"

	"github.com/btcsuite/btcd/wire"
	"github.com/jakm/btcutil/chaincfg"
)

const (
	MainnetMagic wire.BitcoinNet = 0xdab6c3fa
)

var (
	MainNetParams chaincfg.Params
)

func init() {
	MainNetParams = chaincfg.MainNetParams
	MainNetParams.Net = MainnetMagic
	MainNetParams.PubKeyHashAddrID = []byte{30}
	MainNetParams.ScriptHashAddrID = []byte{63}
	MainNetParams.Bech32HRPSegwit = "dgb"
}

// DigiByteParser handle
type DigiByteParser struct {
	*btc.BitcoinParser
}

// NewDigiByteParser returns new VertcoinParser instance
func NewDigiByteParser(params *chaincfg.Params, c *btc.Configuration) *DigiByteParser {
	return &DigiByteParser{BitcoinParser: btc.NewBitcoinParser(params, c)}
}

// GetChainParams contains network parameters for the main DigiByte network
func GetChainParams(chain string) *chaincfg.Params {
	if !chaincfg.IsRegistered(&MainNetParams) {
		err := chaincfg.Register(&MainNetParams)
		if err != nil {
			panic(err)
		}
	}
	return &MainNetParams
}
