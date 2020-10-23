// +build unittest

package kumacoin

import (
	"encoding/hex"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/martinboehm/btcutil/chaincfg"
	"github.com/trezor/blockbook/bchain"
	"github.com/trezor/blockbook/bchain/coins/btc"
)

func TestMain(m *testing.M) {
	c := m.Run()
	chaincfg.ResetParams()
	os.Exit(c)
}

var (
	testTx1       bchain.Tx
	testTxPacked1 = "0a20e5c19f5c8bfa4bf5f2ae997bda372803191c742bc7b9e59817c654422d29fd6a18f5ed9de5052000288ebb950132770a001220833d3e375a8ca1a065c3b89dbcd6c16a35b413d0d317b78e53062b16a25ba9b9180122494830450221009dc7276997582b5c0f881a3dca30e61b04746ddcdcb8a12a1c02b7c88235404102207a6d44b9bcc9843692a11b491718efe72849d9b53d2aa02847a652c9d76703390128ffffffff0f3a0210003a520a051cfb02e82010011a2321023cf11defef04af2dc8668717e7c095f664742a57f900563391b00ae14502978eac22224b456e4e754150354242793537716f756b314d4b6358445268323753576e663961514001"
)

func init() {
	testTx1 = bchain.Tx{
		Blocktime: 1554478837,
		Time:      1554478837,
		Txid:      "e5c19f5c8bfa4bf5f2ae997bda372803191c742bc7b9e59817c654422d29fd6a",
		Version:   1,
		Vin: []bchain.Vin{
			{
				ScriptSig: bchain.ScriptSig{
					Hex: "4830450221009dc7276997582b5c0f881a3dca30e61b04746ddcdcb8a12a1c02b7c88235404102207a6d44b9bcc9843692a11b491718efe72849d9b53d2aa02847a652c9d767033901",
				},
				Txid:     "833d3e375a8ca1a065c3b89dbcd6c16a35b413d0d317b78e53062b16a25ba9b9",
				Vout:     1,
				Sequence: 4294967295,
			},
		},
		Vout: []bchain.Vout{
			{
				ValueSat: *big.NewInt(0),
				N:        0,
				ScriptPubKey: bchain.ScriptPubKey{
					Hex: "",
				},
			},
			{
				ValueSat: *big.NewInt(124470356000),
				N:        1,
				ScriptPubKey: bchain.ScriptPubKey{
					Hex: "21023cf11defef04af2dc8668717e7c095f664742a57f900563391b00ae14502978eac",
					Addresses: []string{
						"KEnNuAP5BBy57qouk1MKcXDRh27SWnf9aQ",
					},
				},
			},
		},
	}

}

func Test_PackTx(t *testing.T) {
	type args struct {
		tx        bchain.Tx
		height    uint32
		blockTime int64
		parser    *KumacoinParser
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "kumacoin-1",
			args: args{
				tx:        testTx1,
				height:    2448782,
				blockTime: 1554478837,
				parser:    NewKumacoinParser(GetChainParams("main"), &btc.Configuration{}),
			},
			want:    testTxPacked1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.args.parser.PackTx(&tt.args.tx, tt.args.height, tt.args.blockTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("packTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			h := hex.EncodeToString(got)
			if !reflect.DeepEqual(h, tt.want) {
				t.Errorf("packTx() = %v, want %v", h, tt.want)
			}
		})
	}
}

func Test_UnpackTx(t *testing.T) {
	type args struct {
		packedTx string
		parser   *KumacoinParser
	}
	tests := []struct {
		name    string
		args    args
		want    *bchain.Tx
		want1   uint32
		wantErr bool
	}{
		{
			name: "kumacoin-1",
			args: args{
				packedTx: testTxPacked1,
				parser:   NewKumacoinParser(GetChainParams("main"), &btc.Configuration{}),
			},
			want:    &testTx1,
			want1:   2448782,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := hex.DecodeString(tt.args.packedTx)
			got, got1, err := tt.args.parser.UnpackTx(b)
			if (err != nil) != tt.wantErr {
				t.Errorf("unpackTx() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unpackTx() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("unpackTx() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
