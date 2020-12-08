package eth

import (
	"bjut_eth/core/types"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"bjut_eth/common"
	"bjut_eth/p2p"
	"gopkg.in/fatih/set.v0"
)

const (
	// 限制已知交易和区块的数量为防止Dos攻击
	// maxKnownTxs 最大已知交易数量
	maxKnownTxs = 32768
	// maxKnownBlocks 最大已知区块数量
	maxKnownBlocks = 1024

	// maxQueuedTxs 待传播的交易队列最大长度
	maxQueuedTxs = 128

	// maxQueuedProps 待传播的区块队列的最大长度 在以太坊中该值为4 因为可能会存在叔父区块 但在修改共识后应当只需要传播最新的区块
	maxQueuedProps = 4

	// maxQueuedAnns 待告知的区块队列的最大长度 同 maxQueuedProps 原理相同
	maxQueuedAnns = 4

	handshakeTimeout = 5 * time.Second
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

// PeerInfo 代表在协议中一个链接节点的主要信息
type PeerInfo struct {
	// Ethereum 协议版本
	Version int `json:"version"`
	// 总难度
	Difficulty *big.Int `json:"difficulty"`
	// 一个节点中最长的区块的哈希
	Head string `json:"head"`
}

// propEvent is a block propagation, waiting for its turn in the broadcast queue.
type propEvent struct {
	block *types.Block
	td    *big.Int
}

// peer 代表一个以太坊全节点中存储的节点信息
type peer struct {
	// 节点ID截取前8位
	id string
	// p2p
	*p2p.Peer
	// 协议版本
	version int
	// 消息读写器
	rw p2p.MsgReadWriter
	// 最高高度区块哈希
	head common.Hash
	// 最高高度区块总难度值
	td *big.Int
	// 读写锁
	lock sync.RWMutex

	// 该 peer 已知的交易哈希集合
	knownTxs set.Interface
	// 该 peer 已知的区块哈希集合
	knownBlocks set.Interface
	// 等待向该 peer 广播的交易队列
	queuedTxs chan []*types.Transaction
	// 等待向该 peer 传播的区块队列
	queuedProps chan *propEvent
	// 等待向该 peer 告知的区块队列
	queuedAnns chan *types.Block
	// 终止传播循环的通道
	term chan struct{}
}

func newPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	return &peer{
		Peer:        p,
		rw:          rw,
		version:     version,
		id:          fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		knownTxs:    set.New(set.NonThreadSafe),
		knownBlocks: set.New(set.NonThreadSafe),
		queuedTxs:   make(chan []*types.Transaction, maxQueuedTxs),
		queuedProps: make(chan *propEvent, maxQueuedProps),
		queuedAnns:  make(chan *types.Block, maxQueuedAnns),
		term:        make(chan struct{}),
	}
}

// broadcast 该循环用于向远程的节点 peer 传播和发布区块 广播交易
// 使用协程执行
func (p *peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				return
			}
			p.Log().Trace("Broadcast transactions", "count", len(txs))

		case prop := <-p.queuedProps:
			if err := p.SendNewBlock(prop.block, prop.td); err != nil {
				return
			}
			p.Log().Trace("Propagated block", "number", prop.block.Number(), "hash", prop.block.Hash(), "td", prop.td)

		case block := <-p.queuedAnns:
			if err := p.SendNewBlockHashes([]common.Hash{block.Hash()}, []uint64{block.NumberU64()}); err != nil {
				return
			}
			p.Log().Trace("Announced block", "number", block.Number(), "hash", block.Hash())

		case <-p.term:
			return
		}
	}
}

// close 停止 broadcast
func (p *peer) close() {
	close(p.term)
}

// Info 返回寄链接节点的信息
func (p *peer) Info() *PeerInfo {
	hash, td := p.Head()

	return &PeerInfo{
		Version:    p.version,
		Difficulty: td,
		Head:       hash.Hex(),
	}
}

// Head 获取链接节点的最长块信息 这里返回的值拷贝
func (p *peer) Head() (hash common.Hash, td *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.head[:])
	return hash, new(big.Int).Set(p.td)
}

// SetHead 更新链接节点的最长块信息
func (p *peer) SetHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.head[:], hash[:])
	p.td.Set(td)
}

// MarkTransaction 将交易哈希加入已知交易集合 防止对该节点重复传播交易
func (p *peer) MarkTransaction(hash common.Hash) {
	// 如果已知交易集合超过最大长度 先将最先进入的交易抛弃
	for p.knownTxs.Size() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

// SendTransactions 最终执行p2p消息发送 向对应的节点发送交易 并记录发送结果
func (p *peer) SendTransactions(txs types.Transactions) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p2p.Send(p.rw, TxMsg, txs)
}

// AsyncSendTransactions 异步发送交易 通过通道实现异步 一般在对 peerSet 遍历时使用 将交易加入待发送交易队列 异步方法在队列满时会丢弃该交易
// 注意 select default 当queuedTxs通道已满时 会执行default 相当于略过了这次添加
func (p *peer) AsyncSendTransactions(txs []*types.Transaction) {
	select {
	case p.queuedTxs <- txs:
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
	default:
		p.Log().Debug("Dropping transaction propagation", "count", len(txs))
	}
}

// SendNewBlockHashes 向对应的节点发送单个或多个区块的信息 告知其可获得的区块
func (p *peer) SendNewBlockHashes(hashes []common.Hash, numbers []uint64) error {
	for _, hash := range hashes {
		p.knownBlocks.Add(hash)
	}
	request := make(newBlockHashesData, len(hashes))
	for i := 0; i < len(hashes); i++ {
		request[i].Hash = hashes[i]
		request[i].Number = numbers[i]
	}
	return p2p.Send(p.rw, NewBlockHashesMsg, request)
}

// AsyncSendNewBlockHash 异步发送区块哈希
func (p *peer) AsyncSendNewBlockHash(block *types.Block) {
	select {
	case p.queuedAnns <- block:
		p.knownBlocks.Add(block.Hash())
	default:
		p.Log().Debug("Dropping block announcement", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// SendNewBlock 向对应的节点发送完整的区块
func (p *peer) SendNewBlock(block *types.Block, td *big.Int) error {
	p.knownBlocks.Add(block.Hash())
	return p2p.Send(p.rw, NewBlockMsg, []interface{}{block, td})
}

// AsyncSendNewBlock 异步发送完整区块
func (p *peer) AsyncSendNewBlock(block *types.Block, td *big.Int) {
	select {
	case p.queuedProps <- &propEvent{block: block, td: td}:
		p.knownBlocks.Add(block.Hash())
	default:
		p.Log().Debug("Dropping block propagation", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// Handshake executes the eth protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
// Handshake 进行以太坊的握手协议 交流协议版本 网络id 头部区块哈希 创世区块 信息
func (p *peer) Handshake(network uint64, td *big.Int, head common.Hash, genesis common.Hash) error {
	// 发送和读取分别开启两个协程 errc用于接收结果
	errc := make(chan error, 2)
	var status statusData

	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status, genesis)
	}()
	// 节点握手超时设置
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	// 需要完成发送和接收两个结果
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			return p2p.DiscReadTimeout
		}
	}
	// 记录头部区块
	p.head = status.CurrentBlock
	return nil
}

// readStatus 读取 StatusMsg 并检查信息
func (p *peer) readStatus(network uint64, status *statusData, genesis common.Hash) (err error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	// 接受到的消息码错误
	if msg.Code != StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	// 超过消息大小限制错误
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	// 解码消息获得 statusData
	if err := msg.Decode(&status); err != nil {
		return errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	// 创世区块需要一致
	if status.GenesisBlock != genesis {
		return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisBlock[:8], genesis[:8])
	}
	// 属于同一网络
	if status.NetworkId != network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	// 协议版本相同
	if int(status.ProtocolVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// peerSet 存储当前活跃的节点 peers 集合
type peerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet 创建保存节点集合
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}

// Register 注册新的链接节点 当该节点已经注册时返回错误 注册后的节点即开启 broadcast
func (ps *peerSet) Register(p *peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	go p.broadcast()

	return nil
}

// Unregister 从活跃的节点集合中删除某节点
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer 根据id查找节点信息
func (ps *peerSet) Peer(id string) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len 返回当前节点集合长度
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// PeersWithoutBlock 返回未发送过给定区块的节点集合
func (ps *peerSet) PeersWithoutBlock(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownBlocks.Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTx 返回未发送过给定交易的节点集合
func (ps *peerSet) PeersWithoutTx(hash common.Hash) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.knownTxs.Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// Close 断开所有链接 closed停止注册
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}
