package ethapi

import (
	"bjut_eth/accounts"
	"bjut_eth/ethdb"
	"bjut_eth/params"
	"context"

	"bjut_eth/common"
	"bjut_eth/core/types"
	"bjut_eth/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// General Ethereum API
	ProtocolVersion() int
	ChainDb() ethdb.Database
	AccountManager() *accounts.Manager
	// BlockChain API
	//HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error)

	// TxPool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)

	ChainConfig() *params.ChainConfig
	CurrentBlock() *types.Block
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)
	return []rpc.API{
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewPublicTransactionPoolAPI(apiBackend, nonceLock),
			Public:    true,
		},
		{
			Namespace: "personal",
			Version:   "1.0",
			Service:   NewPrivateAccountAPI(apiBackend, nonceLock),
			Public:    false,
		},
	}
}
