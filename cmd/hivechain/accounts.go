package main

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var knownAccounts = []genAccount{
	{
		key:  mustParseKey("4552dbe6ca4699322b5d923d0c9bcdd24644f5db8bf89a085b67c6c49b8a1b91"),
		addr: common.HexToAddress("0x7435ed30A8b4AEb0877CEf0c6E8cFFe834eb865f"),
	},
	{
		key:  mustParseKey("f6a8f1603b8368f3ca373292b7310c53bec7b508aecacd442554ebc1c5d0c856"),
		addr: common.HexToAddress("0x84E75c28348fB86AceA1A93a39426d7D60f4CC46"),
	},
	{
		key:  mustParseKey("6e1e16a9c15641c73bf6e237f9293ab1d4e7c12b9adf83cfc94bcf969670f72d"),
		addr: common.HexToAddress("0x4ddE844b71bcdf95512Fb4Dc94e84FB67b512eD8"),
	},
	{
		key:  mustParseKey("fc39d1c9ddbba176d806ebb42d7460189fe56ca163ad3eb6143bfc6beb6f6f72"),
		addr: common.HexToAddress("0xd803681E487E6AC18053aFc5a6cD813c86Ec3E4D"),
	},
	{
		key:  mustParseKey("a88293fefc623644969e2ce6919fb0dbd0fd64f640293b4bf7e1a81c97e7fc7f"),
		addr: common.HexToAddress("0x4a0f1452281bCec5bd90c3dce6162a5995bfe9df"),
	},
	{
		key:  mustParseKey("457075f6822ac29481154792f65c5f1ec335b4fea9ca20f3fea8fa1d78a12c68"),
		addr: common.HexToAddress("0x14e46043e63D0E3cdcf2530519f4cFAf35058Cb2"),
	},
	{
		key:  mustParseKey("9ee3fd550664b246ad7cdba07162dd25530a3b1d51476dd1d85bbc29f0592684"),
		addr: common.HexToAddress("0xE7d13f7Aa2A838D24c59b40186a0aCa1e21CffCC"),
	},
	{
		key:  mustParseKey("865898edcf43206d138c93f1bbd86311f4657b057658558888aa5ac4309626a6"),
		addr: common.HexToAddress("0x16c57eDF7Fa9D9525378B0b81Bf8A3cEd0620C1c"),
	},
	{
		key:  mustParseKey("19168cd7767604b3d19b99dc3da1302b9ccb6ee9ad61660859e07acd4a2625dd"),
		addr: common.HexToAddress("0x2D389075BE5be9F2246Ad654cE152cF05990b209"),
	},
	{
		key:  mustParseKey("ee7f7875d826d7443ccc5c174e38b2c436095018774248a8074ee92d8914dcdb"),
		addr: common.HexToAddress("0x1F4924B14F34e24159387C0A4CdBaa32f3DDb0cF"),
	},
	{
		key:  mustParseKey("bfcd0e032489319f4e5ca03e643b2025db624be6cf99cbfed90c4502e3754850"),
		addr: common.HexToAddress("0x0c2c51a0990AeE1d73C1228de158688341557508"),
	},
	{
		key:  mustParseKey("41be4e00aac79f7ffbb3455053ec05e971645440d594c047cdcc56a3c7458bd6"),
		addr: common.HexToAddress("0x5f552da00dFB4d3749D9e62dCeE3c918855A86A0"),
	},
	{
		key:  mustParseKey("71aa7d299c7607dabfc3d0e5213d612b5e4a97455b596c2f642daac43fa5eeaa"),
		addr: common.HexToAddress("0x3aE75c08b4c907EB63a8960c45B86E1e9ab6123c"),
	},
	{
		key:  mustParseKey("c825f31cd8792851e33a290b3d749e553983111fc1f36dfbbdb45f101973f6a9"),
		addr: common.HexToAddress("0x654aa64f5FbEFb84c270eC74211B81cA8C44A72e"),
	},
	{
		key:  mustParseKey("8d0faa04ae0f9bc3cd4c890aa025d5f40916f4729538b19471c0beefe11d9e19"),
		addr: common.HexToAddress("0x717f8AA2b982BeE0e29f573D31Df288663e1Ce16"),
	},
	{
		key:  mustParseKey("47f666f20e2175606355acec0ea1b37870c15e5797e962340da7ad7972a537e8"),
		addr: common.HexToAddress("0x4340Ee1b812ACB40a1eb561C019c327b243b92Df"),
	},
	{
		key:  mustParseKey("8d56bcbcf2c1b7109e1396a28d7a0234e33544ade74ea32c460ce4a443b239b1"),
		addr: common.HexToAddress("0xC7B99a164Efd027a93f147376Cc7DA7C67c6bbE0"),
	},
	{
		key:  mustParseKey("34391cbbf06956bb506f45ec179cdd84df526aa364e27bbde65db9c15d866d00"),
		addr: common.HexToAddress("0x83C7e323d189f18725ac510004fdC2941F8C4A78"),
	},
	{
		key:  mustParseKey("25e6ce8611cefb5cd338aeaa9292ed2139714668d123a4fb156cabb42051b5b7"),
		addr: common.HexToAddress("0x1F5BDe34B4afC686f136c7a3CB6EC376F7357759"),
	},
	mod7702Account,
}

func mustParseKey(s string) *ecdsa.PrivateKey {
	key, err := crypto.HexToECDSA(s)
	if err != nil {
		panic(err)
	}
	return key
}
