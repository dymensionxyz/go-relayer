package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var ModuleName = "lightclient"

var (
	//
	amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewAminoCodec(amino)
	//
)

//	func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
//		cdc.RegisterConcrete(&MsgSetCanonicalClient{}, "/dym.lightclient.MsgSetCanonicalClient", nil)
//	}
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgSetCanonicalClient{},
	)
}

//}
//
//func init() {
//	//RegisterCodec(Amino)
//	// Register all Amino interfaces and concrete types on the authz Amino codec so that this can later be
//	// used to properly serialize MsgGrant and MsgExec instances
//	RegisterLegacyAminoCodec(amino)
//	//sdk.RegisterLegacyAminoCodec(amino)
//	cryptocodec.RegisterCrypto(amino)
//	//RegisterCodec(authzcodec.Amino)
//
//	amino.Seal()
//}
