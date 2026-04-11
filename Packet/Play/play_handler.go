// Package Play 1.12.2 Play 状态数据包处理
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/command"
	"RootreeMC/entity"
	"RootreeMC/player"
	"RootreeMC/world"
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	// 玩家区块位置追踪
	playerChunkPos = make(map[*Network.Network]struct{ X, Z int32 })
	// 已发送的区块记录 (用于卸载)
	sentChunks = make(map[*Network.Network]map[struct{ X, Z int32 }]bool)
	// 锁，保证地图访问安全
	chunkStateMu sync.Mutex
)

// HandleChatMessage 处理聊天消息
func HandleChatMessage(client *Network.Network, data []byte, username string) {
	if len(data) < 1 {
		return
	}

	// 使用标准协议工具读取字符串，自动处理 VarInt 长度前缀
	reader := bytes.NewReader(data)
	message, err := Protocol.ReadString(reader)
	if err != nil {
		fmt.Printf("[Chat] 解析消息失败: %v\n", err)
		return
	}

	fmt.Printf("[Chat] 玩家发送消息: %s\n", message)

	// 检查是否是命令
	if len(message) > 0 && message[0] == '/' {
		// 使用 command 包处理命令
		p := player.GlobalPlayerManager.GetPlayerByClient(client)
		if p != nil {
			if command.Handle(p, message) {
				fmt.Printf("[Command] %s 执行命令: %s\n", username, message)
				return // 命令执行成功，不广播
			}
		}
		// 命令无效，发送错误消息
		errMsg := BuildSystemMessage(fmt.Sprintf("§c未知命令或参数错误: %s", message))
		client.Send(errMsg)
		return
	}

	// 广播给所有玩家，并带上正确的用户名
	chatMsg := fmt.Sprintf("§7[%s] §r%s", username, message)
	if len(chatMsg) > 100 {
		chatMsg = chatMsg[:97] + "..."
	}
	broadcast := BuildSystemMessage(chatMsg)

	// 广播给所有在线玩家（包括自己）
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, p := range allPlayers {
		p.Client.Send(broadcast)
	}
}

// HandleClientStatus 处理客户端状态
func HandleClientStatus(client *Network.Network, data []byte) {
	if len(data) < 1 {
		return
	}

	actionID := data[0]
	switch actionID {
	case 0:
		fmt.Println("[Play] 客户端请求重生")
	case 1:
		fmt.Println("[Play] 抢夺数据")
	case 2:
		fmt.Println("[Play] 初始化统计完成")
	case 3:
		fmt.Println("[Play] 客户端列表已初始化")
	}
}

// HandleClientSettings 处理客户端设置 (修正为 1.12.2 格式)
func HandleClientSettings(client *Network.Network, data []byte) {
	reader := bytes.NewReader(data)
	locale, err := Protocol.ReadString(reader)
	if err != nil {
		return
	}
	viewDist, _ := reader.ReadByte()
	chatMode, _ := Network.ReadVarint(reader)
	chatColors, _ := reader.ReadByte()
	skinParts, _ := reader.ReadByte() // 这是关键字段：Displayed Skin Parts
	mainHand, _ := Network.ReadVarint(reader)

	fmt.Printf("[Play] 客户端设置: locale=%s, viewDistance=%d, chatMode=%d, chatColors=%d, skinParts=%d, mainHand=%d\n",
		locale, viewDist, chatMode, chatColors, skinParts, mainHand)

	// 获取玩家
	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p == nil {
		return
	}

	// 更新玩家实体的皮肤元数据 (Index 13)
	p.PlayerEntity.Mu.Lock()
	p.PlayerEntity.Metadata[13] = entity.EntityMetadata{
		Index: 13,
		Type:  0, // Byte
		Value: skinParts,
	}
	p.PlayerEntity.Mu.Unlock()

	// 广播元数据更新给所有在线玩家（包括自己）
	// Packet ID: 0x3C (Entity Metadata)
	metaBuf := &bytes.Buffer{}
	Network.WriteVarint(metaBuf, 0x3C)
	Network.WriteVarint(metaBuf, p.PlayerEntity.EID)

	p.PlayerEntity.Mu.RLock()
	entity.BuildEntityMetadata(metaBuf, p.PlayerEntity.Metadata)
	p.PlayerEntity.Mu.RUnlock()

	packet := Protocol.AddLengthPrefix(metaBuf)

	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, online := range allPlayers {
		online.Client.Send(packet)
	}
}

// HandleClientCapabilities 处理客户端能力
func HandleClientCapabilities(client *Network.Network, data []byte) {
	// 客户端能力信息，不需要回复
	_ = len(data)
}

// HandleKeepAlive 处理 Keep Alive (客户端->服务器)
func HandleKeepAlive(client *Network.Network, data []byte) {
	if len(data) < 8 {
		return
	}

	// 解析客户端返回的KeepAliveID
	reader := bytes.NewReader(data)
	keepAliveID, err := Protocol.ReadLong(reader)
	if err != nil {
		return
	}

	// 验证KeepAliveID（与最后发送的ID匹配）
	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p != nil {
		// 更新最后收到Keep Alive的时间
		player.UpdateLastKeepAliveTime(p.PlayerEntity.EID)
		fmt.Printf("[KeepAlive] 收到玩家 %s 的Keep Alive响应 (ID=%d)\n", p.Username, keepAliveID)
	}
}

// HandleAnimation 处理玩家动画 (0x06 Serverbound)
// 当玩家左键攻击或空挥时发送
func HandleAnimation(client *Network.Network, data []byte, username string) {
	if len(data) < 1 {
		return
	}

	// 读取 Hand (VarInt)
	reader := bytes.NewReader(data)
	hand, _ := Network.ReadVarint(reader) // 0=主手, 1=副手

	// 获取玩家实体
	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p == nil {
		return
	}

	// 向其他在线玩家广播摆臂动画
	// animationID=0 表示摆动主臂 (主手攻击)
	// animationID=3 表示摆动副手 (如果hand==1)
	animationID := byte(0) // 默认主手
	if hand == 1 {
		animationID = 3 // 副手摆动
	}

	// 广播给其他玩家（不包括自己）
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	animPkt := entity.BuildEntityAnimation(p.PlayerEntity.EID, animationID)

	// 快速发送动画包，不记录详细日志以减少延迟
	for _, other := range allPlayers {
		if other.UUID != p.UUID {
			other.Client.Send(animPkt)
		}
	}
}

// HandlePlayerPosition 处理玩家位置（优化：添加移动阈值）
func HandlePlayerPosition(client *Network.Network, data []byte) {
	if len(data) < 25 {
		return
	}
	reader := bytes.NewReader(data)
	var x, y, z float64
	var onGround bool
	if err := binary.Read(reader, binary.BigEndian, &x); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &y); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &z); err != nil {
		return
	}
	b, _ := reader.ReadByte()
	onGround = b != 0

	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p != nil {
		// 计算移动距离
		dx := math.Abs(p.PlayerEntity.X - x)
		dy := math.Abs(p.PlayerEntity.Y - y)
		dz := math.Abs(p.PlayerEntity.Z - z)

		// 如果移动很小（< 0.1方块），减少发包频率
		if dx < 0.1 && dy < 0.1 && dz < 0.1 {
			// 微小移动，只更新位置不发广播
			p.PlayerEntity.X = x
			p.PlayerEntity.Y = y
			p.PlayerEntity.Z = z
			p.PlayerEntity.OnGround = onGround
			return
		}

		p.PlayerEntity.X = x
		p.PlayerEntity.Y = y
		p.PlayerEntity.Z = z
		p.PlayerEntity.OnGround = onGround
		// 立即广播移动
		broadcastMovement(p)
		UpdateChunksForPlayer(client, x, z)
	}
}

// HandlePlayerLook 处理玩家视角（优化：快速响应视角变化）
func HandlePlayerLook(client *Network.Network, data []byte) {
	if len(data) < 9 {
		return
	}
	reader := bytes.NewReader(data)
	var yaw, pitch float32
	var onGround bool
	if err := binary.Read(reader, binary.BigEndian, &yaw); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &pitch); err != nil {
		return
	}
	b, _ := reader.ReadByte()
	onGround = b != 0

	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p != nil {
		p.PlayerEntity.Yaw = yaw
		p.PlayerEntity.Pitch = pitch
		p.PlayerEntity.OnGround = onGround
		// 视角变化时立即广播，不延迟
		broadcastMovement(p)
	}
}

// HandlePlayerPositionAndLook 处理玩家位置+视角
func HandlePlayerPositionAndLook(client *Network.Network, data []byte) {
	if len(data) < 33 {
		return
	}
	reader := bytes.NewReader(data)
	var x, y, z float64
	var yaw, pitch float32
	var onGround bool
	if err := binary.Read(reader, binary.BigEndian, &x); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &y); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &z); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &yaw); err != nil {
		return
	}
	if err := binary.Read(reader, binary.BigEndian, &pitch); err != nil {
		return
	}
	b, _ := reader.ReadByte()
	onGround = b != 0

	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p != nil {
		p.PlayerEntity.X = x
		p.PlayerEntity.Y = y
		p.PlayerEntity.Z = z
		p.PlayerEntity.Yaw = yaw
		p.PlayerEntity.Pitch = pitch
		p.PlayerEntity.OnGround = onGround
		broadcastMovement(p)
		UpdateChunksForPlayer(client, x, z)
	}
}

// broadcastMovement 向其他在线玩家广播运动
func broadcastMovement(p *player.OnlinePlayer) {
	teleportPkt := entity.BuildEntityTeleport(
		p.PlayerEntity.EID,
		p.PlayerEntity.X,
		p.PlayerEntity.Y,
		p.PlayerEntity.Z,
		p.PlayerEntity.Yaw,
		p.PlayerEntity.Pitch,
		p.PlayerEntity.OnGround,
	)
	headLookPkt := entity.BuildEntityHeadLook(p.PlayerEntity.EID, p.PlayerEntity.Yaw)

	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, other := range allPlayers {
		if other.UUID != p.UUID {
			other.Client.Send(teleportPkt)
			other.Client.Send(headLookPkt)
		}
	}
}

// HandlePlayerDigging 处理玩家挖掘
func HandlePlayerDigging(client *Network.Network, data []byte) {
	if len(data) < 14 {
		return
	}

	status := data[0]
	location := binary.BigEndian.Uint64(data[1:9])
	x := int32(location >> 38)
	y := int32((location << 26) >> 52)
	z := int32((location << 38) >> 38)

	// 只在完成挖掘时破坏方块
	if status == 2 {
		p := player.GlobalPlayerManager.GetPlayerByClient(client)
		canDrop := p != nil && p.PlayerEntity != nil && (p.PlayerEntity.Gamemode == 0 || p.PlayerEntity.Gamemode == 2)

		// 获取当前方块
		currentBlock := world.GlobalWorld.GetBlock(x, y, z)
		if currentBlock != 0 { // 不是空气
			// 设置为空气
			if world.GlobalWorld.SetBlock(x, y, z, 0) {
				// 广播方块更新给所有在线玩家
				updatePkt := world.BuildBlockChange(x, y, z, 0)
				broadcastToAllPlayers(updatePkt)

				if canDrop {
					spawnBlockDropEntity(x, y, z, currentBlock)
				}
			}
		}
	}
}

func spawnBlockDropEntity(x, y, z int32, brokenState uint16) {
	itemID, count, ok := getDropForBrokenBlock(brokenState)
	if !ok || itemID <= 0 || count <= 0 {
		return
	}

	eid := entity.GlobalEntityManager.CreateItemEntity(
		itemID,
		count,
		nil,
		float64(x)+0.5,
		float64(y)+0.5,
		float64(z)+0.5,
		0.0, 0.08, 0.0,
	)
	if eid <= 0 {
		return
	}

	itemEntity := entity.GlobalEntityManager.GetItemEntity(eid)
	if itemEntity == nil {
		return
	}

	broadcastToAllPlayers(entity.BuildSpawnItemEntity(itemEntity))
	broadcastToAllPlayers(BuildItemEntityMetadata(itemEntity))
}

func getDropForBrokenBlock(state uint16) (itemID int32, count int32, ok bool) {
	blockID := world.GetID(state)

	switch blockID {
	case 0, 7, 8, 9, 10, 11, 31, 32, 51, 59:
		return 0, 0, false
	case 1:
		return 4, 1, true // Stone -> Cobblestone
	case 2:
		return 3, 1, true // Grass Block -> Dirt
	case 16:
		return 263, 1, true // Coal Ore -> Coal
	case 56:
		return 264, 1, true // Diamond Ore -> Diamond
	case 73, 74:
		return 331, 4, true // Redstone Ore -> Redstone Dust
	case 62:
		return 61, 1, true // Lit Furnace -> Furnace
	case 63, 68:
		return 323, 1, true // Sign block -> Sign item
	case 64:
		return 324, 1, true // Wooden Door block -> Door item
	case 71:
		return 330, 1, true // Iron Door block -> Door item
	case 78:
		return 332, 1, true // Snow layer -> Snowball
	default:
		return int32(blockID), 1, true
	}
}

// HandlePlayerBlockPlacement 处理玩家放置方块
func HandlePlayerBlockPlacement(client *Network.Network, data []byte) {
	if len(data) < 15 {
		return
	}

	reader := bytes.NewReader(data)

	// 读取 Hand (VarInt)
	hand, _ := Network.ReadVarint(reader)

	// 读取位置 (Position: 8 bytes)
	posBytes := make([]byte, 8)
	if _, err := reader.Read(posBytes); err != nil {
		fmt.Printf("[Play] 读取方块放置位置失败: %v\n", err)
		return
	}
	location := binary.BigEndian.Uint64(posBytes)
	x := int32(location >> 38)
	y := int32((location << 26) >> 52)
	z := int32((location << 38) >> 38)

	// 读取 Face (Byte)
	face, _ := reader.ReadByte()

	// 跳过 Held Item (Slot 数据，暂时不用)
	// Slot 格式: Present(Boolean) | ItemID(VarInt) | Count(Byte) | NBT(Tag)

	fmt.Printf("[Play] 玩家放置方块: hand=%d, pos=(%d,%d,%d), face=%d\n", hand, x, y, z, face)

	// 计算放置位置（根据面）
	placeX, placeY, placeZ := x, y, z
	switch face {
	case 0: // 下面
		placeY--
	case 1: // 上面
		placeY++
	case 2: // 北面
		placeZ--
	case 3: // 南面
		placeZ++
	case 4: // 西面
		placeX--
	case 5: // 东面
		placeX++
	}

	// 放置方块（使用石头）
	blockState := world.ToState(1, 0) // Stone (state = ID<<4|Meta)
	if world.GlobalWorld.SetBlock(placeX, placeY, placeZ, blockState) {
		fmt.Printf("[World] 放置方块: (%d,%d,%d) = state %d\n", placeX, placeY, placeZ, blockState)

		// 发送方块更新给客户端
		updatePkt := world.BuildBlockChange(placeX, placeY, placeZ, blockState)
		client.Send(updatePkt)
	}
}

// HandleEntityAction 处理实体动作
func HandleEntityAction(client *Network.Network, data []byte) {
	if len(data) < 6 {
		return
	}

	reader := bytes.NewReader(data)
	entityID, _ := Network.ReadVarint(reader)
	actionID, _ := Network.ReadVarint(reader)

	actionStr := "未知"
	switch actionID {
	case 0:
		actionStr = "蹲下"
	case 1:
		actionStr = "站起"
	case 2:
		actionStr = "开始奔跑"
	case 3:
		actionStr = "停止奔跑"
	case 4:
		actionStr = "开始跳跃"
	case 5:
		actionStr = "停止跳跃"
	case 6:
		actionStr = "开始休息"
	case 7:
		actionStr = "停止休息"
	}

	fmt.Printf("[Play] 实体动作: entityID=%d, action=%s\n", entityID, actionStr)
}

// HandleClickWindow 处理点击窗口
func HandleClickWindow(client *Network.Network, data []byte) {
	_ = len(data)
}

// HandleCloseWindow 处理关闭窗口
func HandleCloseWindow(client *Network.Network, data []byte) {
	if len(data) < 1 {
		return
	}

	windowID := data[0]
	fmt.Printf("[Play] 关闭窗口: windowID=%d\n", windowID)
}

const ViewDistance = 4 // 4 = 9x9 区块 (中心 + 各方向4个)

// UpdateChunksForPlayer 根据玩家位置更新区块 (添加线程安全保护)
func UpdateChunksForPlayer(client *Network.Network, x, z float64) {
	chunkX := int32(math.Floor(x / 16))
	chunkZ := int32(math.Floor(z / 16))

	chunkStateMu.Lock()
	lastPos, exists := playerChunkPos[client]
	if exists && lastPos.X == chunkX && lastPos.Z == chunkZ {
		chunkStateMu.Unlock()
		return
	}

	playerChunkPos[client] = struct{ X, Z int32 }{chunkX, chunkZ}
	if !exists {
		sentChunks[client] = make(map[struct{ X, Z int32 }]bool)
	}

	// 拷贝一份 sentChunks 以便在持有锁之外发送网络包
	mySentChunks := make(map[struct{ X, Z int32 }]bool)
	for k, v := range sentChunks[client] {
		mySentChunks[k] = v
	}
	chunkStateMu.Unlock()

	fmt.Printf("[Chunk] 玩家进入区块 (%d, %d)\n", chunkX, chunkZ)

	newChunks := make(map[struct{ X, Z int32 }]bool)
	for dx := -ViewDistance; dx <= ViewDistance; dx++ {
		for dz := -ViewDistance; dz <= ViewDistance; dz++ {
			newChunks[struct{ X, Z int32 }{chunkX + int32(dx), chunkZ + int32(dz)}] = true
		}
	}

	for pos := range mySentChunks {
		if !newChunks[pos] {
			unloadPkt := world.BuildChunkUnload(pos.X, pos.Z)
			client.Send(unloadPkt)
			chunkStateMu.Lock()
			delete(sentChunks[client], pos)
			chunkStateMu.Unlock()
		}
	}

	for pos := range newChunks {
		chunkStateMu.Lock()
		isSent := sentChunks[client][pos]
		chunkStateMu.Unlock()

		if !isSent {
			chunk := world.GlobalWorld.GetOrCreateChunk(pos.X, pos.Z)
			packet := world.BuildMapChunk(chunk)
			client.Send(packet)
			
			// 暂时禁用光照更新包，测试是否是它导致的问题
			// lightPkt := world.BuildSimpleLightUpdate(pos.X, pos.Z)
			// client.Send(lightPkt)
			
			chunkStateMu.Lock()
			sentChunks[client][pos] = true
			chunkStateMu.Unlock()
		}
	}
}

// StartKeepAliveLoop 启动 Keep Alive 循环（优化：1秒间隔）
func StartKeepAliveLoop(client *Network.Network) {
	go func() {
		ticker := time.NewTicker(1 * time.Second) // 改为1秒间隔
		defer ticker.Stop()

		for {
			<-ticker.C
			keepAliveID := time.Now().UnixNano()
			packet := BuildKeepAlive(keepAliveID)

			// 记录发送的KeepAliveID和发送时间
			p := player.GlobalPlayerManager.GetPlayerByClient(client)
			if p != nil {
				player.RecordKeepAliveSend(p.PlayerEntity.EID, keepAliveID)
			}

			if err := client.Send(packet); err != nil {
				if p != nil {
					fmt.Printf("[KeepAlive] 发送失败 to %s: %v\n", p.Username, err)
				}
				return
			}
		}
	}()
}
