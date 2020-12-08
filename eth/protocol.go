package eth

import (
	"bjut_eth/common"
	"bjut_eth/core"
	"bjut_eth/core/types"
	"bjut_eth/event"
)

// 协议版本常量
const (
	eth62 = 62
	eth63 = 63
)

// ProtocolName 协议名称缩写
var ProtocolName = "eth"

// 协议版本集合 只采用全节点
var ProtocolVersions = []uint{eth63, eth62}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{17, 8}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// 以太坊协议的消息编码 以太坊中采用全节点和轻节点所以协议区分为两部分 当前修改以全节点为标准
const (
	// Protocol messages belonging to bjut_eth/62
	StatusMsg          = 0x00
	NewBlockHashesMsg  = 0x01
	TxMsg              = 0x02
	GetBlockHeadersMsg = 0x03
	BlockHeadersMsg    = 0x04
	GetBlockBodiesMsg  = 0x05
	BlockBodiesMsg     = 0x06
	NewBlockMsg        = 0x07

	// Protocol messages belonging to bjut_eth/63
	GetNodeDataMsg = 0x0d
	NodeDataMsg    = 0x0e
	GetReceiptsMsg = 0x0f
	ReceiptsMsg    = 0x10
)

type errCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
)

func (e errCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
}

// 没有理解这里创建了txPool的接口的目的 因为协议部分只关注这三个方法？
type txPool interface {
	// AddRemotes 添加接收到的交易
	AddRemotes([]*types.Transaction) []error

	// Pending 返回pending状态的交易
	Pending() (map[common.Address]types.Transactions, error)

	// SubscribeNewTxsEvent 订阅新交易事件
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription
}

// statusData 用于包装网络信息
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
}

// newBlockHashesData 用于包装通知区块信息
type newBlockHashesData []struct {
	Hash   common.Hash
	Number uint64
}
