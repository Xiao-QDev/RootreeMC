// Package player 玩家管理器
package player

import (
	"RootreeMC/Network"
	//"RootreeMC/Protocol"
	"RootreeMC/entity"
	"RootreeMC/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PlayerData 玩家完整数据
type PlayerData struct {
	Username  string
	UUID      string
	Position  PlayerPosition
	Inventory inventory.Inventory
	Gamemode  int32
	LastSeen  int64 // 时间戳
}

// PlayerPosition 玩家位置
type PlayerPosition struct {
	X, Y, Z    float64
	Yaw, Pitch float32
}

// OnlinePlayer 在线玩家（包含实体和网络客户端）
type OnlinePlayer struct {
	PlayerEntity *entity.PlayerEntity
	Client       *Network.Network
	Username     string
	UUID         string
	Inventory    *inventory.Inventory
	Properties   []entity.PlayerProperty // 存储从 Mojang 获取的皮肤等属性
}

// PlayerManager 玩家管理器
type PlayerManager struct {
	players        map[string]*OnlinePlayer    // UUID -> OnlinePlayer
	clientToPlayer map[*Network.Network]string // Client -> UUID
	entityToPlayer map[int32]string            // EID -> UUID
	dataDir        string
	mu             sync.RWMutex
}

// GlobalPlayerManager 全局玩家管理器
var (
	GlobalPlayerManager *PlayerManager
	globalTPS           float64 = 20.0
	statsMu             sync.RWMutex
)

// SetTPS 设置全局 TPS
func SetTPS(tps float64) {
	statsMu.Lock()
	defer statsMu.Unlock()
	globalTPS = tps
}

// GetTPS 获取全局 TPS
func GetTPS() float64 {
	statsMu.RLock()
	defer statsMu.RUnlock()
	return globalTPS
}

func init() {
	GlobalPlayerManager = NewPlayerManager("saves/players")
	
	// 注册世界光照引擎的广播回调（避免循环导入）
	// 使用匿名函数包装，避免直接导入 world 包
	broadcastFunc := func(pkt []byte) {
		allPlayers := GlobalPlayerManager.GetAllOnlinePlayers()
		for _, p := range allPlayers {
			p.Client.Send(pkt)
		}
	}
	
	// 通过 init 函数的副作用来注册回调
	// 因为无法直接导入 world 包，这里假设 world 包会调用 player.RegisterBroadcastCallback
	// 实际注册在 main.go 中进行
	_ = broadcastFunc
}

// NewPlayerManager 创建玩家管理器
func NewPlayerManager(dataDir string) *PlayerManager {
	// 确保数据目录存在
	os.MkdirAll(dataDir, 0755)

	return &PlayerManager{
		players:        make(map[string]*OnlinePlayer),
		clientToPlayer: make(map[*Network.Network]string),
		entityToPlayer: make(map[int32]string),
		dataDir:        dataDir,
	}
}

// PlayerJoin 玩家加入游戏 (添加 props 参数)
func (pm *PlayerManager) PlayerJoin(client *Network.Network, username string, uuidBytes []byte, props []entity.PlayerProperty) *OnlinePlayer {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	uuidStr := fmt.Sprintf("%x", uuidBytes)

	// 加载或创建玩家数据
	playerData := pm.loadPlayerData(uuidStr)
	if playerData == nil {
		// 新玩家
		playerData = &PlayerData{
			Username: username,
			UUID:     uuidStr,
			Position: PlayerPosition{
				X: 8.5, Y: 64, Z: 8.5,
				Yaw: 0, Pitch: 0,
			},
			Inventory: *inventory.NewInventory(),
			Gamemode:  1, // Creative
			LastSeen:  time.Now().Unix(),
		}

		// 给新玩家初始物品（Creative模式）
		pm.giveStarterItems(&playerData.Inventory)
	}

	// 创建玩家实体
	playerEntity := entity.GlobalEntityManager.CreatePlayer(
		username,
		uuidBytes,
		playerData.Position.X,
		playerData.Position.Y,
		playerData.Position.Z,
	)

	// 设置朝向
	playerEntity.Yaw = playerData.Position.Yaw
	playerEntity.Pitch = playerData.Position.Pitch
	
	// 设置皮肤属性
	if len(props) > 0 {
		playerEntity.Properties = props
	}

	// 创建在线玩家
	onlinePlayer := &OnlinePlayer{
		PlayerEntity: playerEntity,
		Client:       client,
		Username:     username,
		UUID:         uuidStr,
		Inventory:    &playerData.Inventory,
		Properties:   props,
	}

	// 存储映射 (注意: pm.mu 已由函数开始处的 defer 锁定)
	pm.players[uuidStr] = onlinePlayer
	pm.clientToPlayer[client] = uuidStr
	pm.entityToPlayer[playerEntity.EID] = uuidStr

	fmt.Printf("[Player] 玩家加入: %s (EID: %d, UUID: %s)\n", username, playerEntity.Entity.EID, uuidStr)

	return onlinePlayer
}

// PlayerLeave 玩家离开游戏
func (pm *PlayerManager) PlayerLeave(client *Network.Network) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	uuid, exists := pm.clientToPlayer[client]
	if !exists {
		return
	}

	player, exists := pm.players[uuid]
	if !exists {
		return
	}

	// 保存玩家数据
	pm.savePlayerData(player)

	// 移除实体
	entity.GlobalEntityManager.RemoveEntity(player.PlayerEntity.Entity.EID)

	// 清理映射
	delete(pm.players, uuid)
	delete(pm.clientToPlayer, client)
	delete(pm.entityToPlayer, player.PlayerEntity.Entity.EID)

	fmt.Printf("[Player] 玩家离开: %s (EID: %d)\n", player.Username, player.PlayerEntity.Entity.EID)
}

// GetPlayerByClient 通过客户端获取玩家
func (pm *PlayerManager) GetPlayerByClient(client *Network.Network) *OnlinePlayer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	uuid, exists := pm.clientToPlayer[client]
	if !exists {
		return nil
	}

	return pm.players[uuid]
}

// GetPlayerByUUID 通过UUID获取玩家
func (pm *PlayerManager) GetPlayerByUUID(uuid string) *OnlinePlayer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.players[uuid]
}

// GetAllOnlinePlayers 获取所有在线玩家
func (pm *PlayerManager) GetAllOnlinePlayers() []*OnlinePlayer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*OnlinePlayer, 0, len(pm.players))
	for _, p := range pm.players {
		result = append(result, p)
	}
	return result
}

// SaveAllPlayers 保存所有在线玩家数据
func (pm *PlayerManager) SaveAllPlayers() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, player := range pm.players {
		pm.savePlayerData(player)
	}
	fmt.Printf("[Player] 保存了 %d 个在线玩家的数据\n", len(pm.players))
}

// 内部函数：加载玩家数据
func (pm *PlayerManager) loadPlayerData(uuid string) *PlayerData {
	filename := filepath.Join(pm.dataDir, uuid+".json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 新玩家
		}
		fmt.Printf("[Player] 读取玩家数据失败 %s: %v\n", uuid, err)
		return nil
	}

	var playerData PlayerData
	err = json.Unmarshal(data, &playerData)
	if err != nil {
		fmt.Printf("[Player] 解析玩家数据失败 %s: %v\n", uuid, err)
		return nil
	}

	fmt.Printf("[Player] 加载玩家数据: %s\n", playerData.Username)
	return &playerData
}

// 内部函数：保存玩家数据
func (pm *PlayerManager) savePlayerData(player *OnlinePlayer) {
	playerData := PlayerData{
		Username: player.Username,
		UUID:     player.UUID,
		Position: PlayerPosition{
			X:     player.PlayerEntity.X,
			Y:     player.PlayerEntity.Y,
			Z:     player.PlayerEntity.Z,
			Yaw:   player.PlayerEntity.Yaw,
			Pitch: player.PlayerEntity.Pitch,
		},
		Inventory: *player.Inventory,
		Gamemode:  player.PlayerEntity.Gamemode,
		LastSeen:  time.Now().Unix(),
	}

	filename := filepath.Join(pm.dataDir, player.UUID+".json")

	data, err := json.MarshalIndent(playerData, "", "  ")
	if err != nil {
		fmt.Printf("[Player] 序列化玩家数据失败 %s: %v\n", player.UUID, err)
		return
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		fmt.Printf("[Player] 写入玩家数据失败 %s: %v\n", player.UUID, err)
		return
	}

	fmt.Printf("[Player] 保存玩家数据: %s\n", player.Username)
}

// 给新玩家初始物品
func (pm *PlayerManager) giveStarterItems(inv *inventory.Inventory) {
	// 快捷栏：各种方块
	inv.SetItem(0, inventory.NewItemStack(inventory.ItemStone, 64))
	inv.SetItem(1, inventory.NewItemStack(inventory.ItemDirt, 64))
	inv.SetItem(2, inventory.NewItemStack(inventory.ItemOakPlanks, 64))
	inv.SetItem(3, inventory.NewItemStack(inventory.ItemGlass, 64))
	inv.SetItem(4, inventory.NewItemStack(inventory.ItemTorch, 64))
	inv.SetItem(5, inventory.NewItemStack(inventory.ItemChest, 64))
	inv.SetItem(6, inventory.NewItemStack(inventory.ItemCraftingTable, 64))
	inv.SetItem(7, inventory.NewItemStack(inventory.ItemFurnace, 64))
	inv.SetItem(8, inventory.NewItemStack(inventory.ItemTNT, 64))

	// 背包：工具
	inv.SetItem(9, inventory.NewItemStack(inventory.ItemIronPickaxe, 1))
	inv.SetItem(10, inventory.NewItemStack(inventory.ItemIronShovel, 1))
	inv.SetItem(11, inventory.NewItemStack(inventory.ItemIronAxe, 1))
	inv.SetItem(12, inventory.NewItemStack(inventory.ItemFlintAndSteel, 1))
}

// NewItemStack 创建新的物品堆叠（导出函数，供command包使用）
func NewItemStack(itemID int32, count byte) inventory.ItemStack {
	return inventory.ItemStack{
		ItemID: itemID,
		Count:  count,
		Damage: 0,
	}
}

// BroadcastAnimation 向其他在线玩家广播实体动画
// animationID: 0=摆动主臂, 1=受到伤害, 2=离开床, 3=摆动副手, 4=临界效应, 5=魔法临界效应
func BroadcastAnimation(eid int32, animationID byte, excludeEID int32) {
	animPkt := entity.BuildEntityAnimation(eid, animationID)
	
	allPlayers := GlobalPlayerManager.GetAllOnlinePlayers()
	for _, p := range allPlayers {
		if p.PlayerEntity.EID != excludeEID {
			p.Client.Send(animPkt)
		}
	}
}

// ====== Keep Alive 追踪 ======

type keepAliveRecord struct {
	lastSendTime    int64 // 最后发送Keep Alive的时间戳（UnixNano）
	lastReceiveTime int64 // 最后收到Keep Alive响应的时间戳
	lastKeepAliveID int64 // 最后发送的Keep Alive ID
}

var (
	keepAliveMap = make(map[int32]*keepAliveRecord) // EID -> Keep Alive记录
	keepAliveMu  sync.RWMutex
)

// RecordKeepAliveSend 记录Keep Alive发送
func RecordKeepAliveSend(eid int32, keepAliveID int64) {
	keepAliveMu.Lock()
	defer keepAliveMu.Unlock()
	
	if _, ok := keepAliveMap[eid]; !ok {
		keepAliveMap[eid] = &keepAliveRecord{}
	}
	keepAliveMap[eid].lastSendTime = time.Now().UnixNano()
	keepAliveMap[eid].lastKeepAliveID = keepAliveID
}

// UpdateLastKeepAliveTime 更新最后收到Keep Alive的时间
func UpdateLastKeepAliveTime(eid int32) {
	keepAliveMu.Lock()
	defer keepAliveMu.Unlock()
	
	if record, ok := keepAliveMap[eid]; ok {
		record.lastReceiveTime = time.Now().UnixNano()
	}
}

// CheckKeepAliveTimeout 检查Keep Alive超时（应在tick中调用）
func CheckKeepAliveTimeout() {
	keepAliveMu.RLock()
	defer keepAliveMu.RUnlock()
	
	now := time.Now().UnixNano()
	timeout := int64(30 * time.Second) // 30秒超时
	
	for eid, record := range keepAliveMap {
		// 如果超过30秒没收到响应，断开连接
		if now-record.lastReceiveTime > timeout {
			players := GlobalPlayerManager.GetAllOnlinePlayers()
			for _, player := range players {
				if player.PlayerEntity.EID == eid {
					fmt.Printf("[KeepAlive] 玩家 %s Keep Alive超时，断开连接\n", player.Username)
					player.Client.Close()
					break
				}
			}
		}
	}
}

// RemoveKeepAliveRecord 移除Keep Alive记录（玩家离开时调用）
func RemoveKeepAliveRecord(eid int32) {
	keepAliveMu.Lock()
	defer keepAliveMu.Unlock()
	delete(keepAliveMap, eid)
}

// ====== entity.Player 接口实现 ======

// GetPosition 获取玩家位置 (实现entity.Player接口)
func (p *OnlinePlayer) GetPosition() (float64, float64, float64) {
	p.PlayerEntity.Mu.RLock()
	defer p.PlayerEntity.Mu.RUnlock()
	return p.PlayerEntity.X, p.PlayerEntity.Y, p.PlayerEntity.Z
}

// GetName 获取玩家名称 (实现entity.Player接口)
func (p *OnlinePlayer) GetName() string {
	return p.Username
}

// SendPacket 发送数据包 (实现entity.Player接口)
func (p *OnlinePlayer) SendPacket(data []byte) error {
	return p.Client.Send(data)
}
