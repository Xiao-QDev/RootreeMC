// Package Packet 数据包管理器
package Packet

import (
	"RootreeMC/Network"
	"RootreeMC/Packet/Play"
	"RootreeMC/Packet/Status"
	"RootreeMC/Protocol"
	"RootreeMC/Tick"
	"RootreeMC/Uuid"
	"RootreeMC/entity"
	"RootreeMC/player"
	"RootreeMC/world"
	"fmt"
	"sync"
)

var (
	// 客户端用户名映射
	clientUsernames = make(map[*Network.Network]string)
	clientUserMu    sync.RWMutex
)

// SendGameInitialization 发送游戏初始化数据包序列
func SendGameInitialization(client *Network.Network, username string, pUUID *Uuid.PlayerUUID, props []entity.PlayerProperty) {
	// 使用 PlayerManager 处理玩家加入
	p := player.GlobalPlayerManager.PlayerJoin(client, username, pUUID.Bytes(), props)

	// 存储用户名以便聊天时使用
	clientUserMu.Lock()
	clientUsernames[client] = username
	clientUserMu.Unlock()

	fmt.Printf("[Init] 玩家位置: (%.1f, %.1f, %.1f)\n", p.PlayerEntity.X, p.PlayerEntity.Y, p.PlayerEntity.Z)

	// 1. 发送 Join Game 包
	joinGamePkt := Play.BuildDefaultJoinGame(p.PlayerEntity.Entity.EID)
	_ = client.Send(joinGamePkt)

	// 1.1 发送难度包 (1.12.2 客户端必需)
	diffPkt := Play.BuildDifficulty(1) // Normal
	_ = client.Send(diffPkt)

	// 2. 发送出生点位置
	spawnX, spawnY, spawnZ := world.GlobalWorld.GetSpawnPoint()
	spawnPosPkt := Play.BuildSpawnPosition(spawnX, spawnY, spawnZ)
	_ = client.Send(spawnPosPkt)

	// 3. 发送玩家能力 (根据游戏模式)
	if p.PlayerEntity.Gamemode == 1 {
		abilitiesPkt := Play.BuildCreativeAbilities()
		_ = client.Send(abilitiesPkt)
	}

	// 4. 发送生命值/饱食度
	healthPkt := Play.BuildFullHealth()
	_ = client.Send(healthPkt)

	// 5. 发送时间同步 (1.12.2 客户端必需，用于显示昼夜)
	timePkt := Play.BuildTimeUpdate(Tick.GlobalGameState.WorldAge, Tick.GlobalGameState.TimeOfDay)
	_ = client.Send(timePkt)

	// 6. 发送玩家列表 (简化版本，不包含属性以避免长度问题)
	allOnlinePlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	entries := make([]Play.PlayerListEntry, 0, len(allOnlinePlayers))
	for _, onlineP := range allOnlinePlayers {
		u, _ := Uuid.ParseHex(onlineP.UUID)

		// 不发送任何属性，避免base64字符串过长
		entries = append(entries, Play.PlayerListEntry{
			UUID:        u.Bytes(),
			Name:        onlineP.Username,
			Properties:  []Play.PlayerProperty{}, // 空属性数组
			Gamemode:    onlineP.PlayerEntity.Gamemode,
			Ping:        0,
			DisplayName: "",
		})
	}
	playerListPkt := Play.BuildPlayerListAdd(entries)
	_ = client.Send(playerListPkt)

	// 7. 发送玩家位置和视角
	teleportPkt := Protocol.BuildAbsoluteTeleport(
		p.PlayerEntity.X,
		p.PlayerEntity.Y,
		p.PlayerEntity.Z,
		p.PlayerEntity.Yaw,
		p.PlayerEntity.Pitch,
		0,
	)
	_ = client.Send(teleportPkt)

	// 8. 发送系统消息（新玩家自己的视角）
	sysMsg := Play.BuildSystemMessage(fmt.Sprintf("§e%s joined the game", username))
	_ = client.Send(sysMsg)

	// 9. 向新玩家发送其他在线玩家的生成包
	for _, onlineP := range allOnlinePlayers {
		if onlineP.UUID != p.UUID {
			otherSpawnPkt := entity.BuildSpawnPlayer(onlineP.PlayerEntity)
			_ = client.Send(otherSpawnPkt)
		}
	}

	// 10. 向其他玩家广播新玩家加入（包含加入消息）
	broadcastPlayerJoin(p)
}

func broadcastPlayerJoin(newPlayer *player.OnlinePlayer) {
	// 1. 准备新玩家的列表项 (Add Player)
	newPlayerUUID, _ := Uuid.ParseHex(newPlayer.UUID)

	// 转换属性
	listProps := make([]Play.PlayerProperty, len(newPlayer.Properties))
	for i, lp := range newPlayer.Properties {
		listProps[i] = Play.PlayerProperty{
			Name:      lp.Name,
			Value:     lp.Value,
			Signature: lp.Signature,
			IsSigned:  lp.IsSigned,
		}
	}

	if len(listProps) == 0 {
		listProps = getDefaultSkinProperties(newPlayer.Username)
	}

	newPlayerEntry := Play.PlayerListEntry{
		UUID:        newPlayerUUID.Bytes(),
		Name:        newPlayer.Username,
		Properties:  listProps,
		Gamemode:    newPlayer.PlayerEntity.Gamemode,
		Ping:        0,
		DisplayName: "",
	}
	playerListAddPkt := Play.BuildPlayerListAdd([]Play.PlayerListEntry{newPlayerEntry})
	spawnPkt := entity.BuildSpawnPlayer(newPlayer.PlayerEntity)
	
	// 2. 构建加入消息
	joinMsg := Play.BuildSystemMessage(fmt.Sprintf("§e%s joined the game", newPlayer.Username))

	// 3. 获取所有在线玩家并广播
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, p := range allPlayers {
		if p.UUID != newPlayer.UUID {
			p.Client.Send(playerListAddPkt)
			p.Client.Send(spawnPkt)
			p.Client.Send(joinMsg)
		}
	}
}

// getDefaultSkinProperties 获取默认皮肤属性（启用第二图层）
func getDefaultSkinProperties(username string) []Play.PlayerProperty {
	// 由于皮肤属性的base64编码字符串过长可能导致客户端断开连接，
	// 暂时返回空属性数组。这会导致玩家使用默认皮肤（Steve/Alex），
	// 但不会造成连接问题。
	return []Play.PlayerProperty{}
}

// BroadcastPlayerLeave 向所有其他在线玩家广播玩家离开（销毁实体+列表移除+聊天消息）
func BroadcastPlayerLeave(leavingPlayer *player.OnlinePlayer) {
	broadcastPlayerLeave(leavingPlayer)
}

// CleanupClientUsername 清理断开客户端的用户名映射
func CleanupClientUsername(client *Network.Network) {
	clientUserMu.Lock()
	delete(clientUsernames, client)
	clientUserMu.Unlock()
}
func broadcastPlayerLeave(leavingPlayer *player.OnlinePlayer) {
	eid := leavingPlayer.PlayerEntity.Entity.EID
	username := leavingPlayer.Username

	// 1. 销毁实体包
	destroyPkt := entity.BuildDestroyEntities([]int32{eid})

	// 2. 玩家列表移除包
	leaveUUID, _ := Uuid.ParseHex(leavingPlayer.UUID)
	playerListRemovePkt := Play.BuildPlayerListRemove([][]byte{leaveUUID.Bytes()})

	// 3. 离开聊天消息
	chatMsg := Play.BuildSystemMessage(fmt.Sprintf("%s left the game", username))

	// 4. 广播给所有剩余在线玩家
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, p := range allPlayers {
		if p.UUID != leavingPlayer.UUID {
			p.Client.Send(destroyPkt)
			p.Client.Send(playerListRemovePkt)
			p.Client.Send(chatMsg)
		}
	}

	fmt.Printf("[Broadcast] 玩家离开广播: %s (EID: %d), 剩余在线: %d\n", username, eid, len(allPlayers)-1)
}

func StartKeepAliveLoop(client *Network.Network) {
	Play.StartKeepAliveLoop(client)
}

// 辅助函数：判断区块是否包含任何非空气方块
func hasContent(c *world.Chunk) bool {
	if c == nil {
		return false
	}
	for x := 0; x < 16; x++ {
		for z := 0; z < 16; z++ {
			// 只检查最低处的几个层，提高性能
			if c.Blocks[x][0][z] != 0 || c.Blocks[x][64][z] != 0 {
				return true
			}
		}
	}
	return false
}

func HandleStatusPackets(client *Network.Network, packetID int32, packetData []byte, motd string, maxPlayers int) bool {
	switch packetID {
	case 0x00: // Status Request
		// 动态获取当前在线玩家数
		onlineCount := len(player.GlobalPlayerManager.GetAllOnlinePlayers())

		// 使用统一的状态响应构建工具
		response := Status.BuildStatusResponse(motd, maxPlayers, onlineCount, "1.12.2", 340)
		_ = client.Send(response)
	case 0x01: // Ping Request
		response := Status.BuildPongResponse(packetData)
		_ = client.Send(response)
		return false
	}
	return true
}

func HandlePlayPackets(client *Network.Network, packetID int32, packetData []byte) {
	clientUserMu.RLock()
	username := clientUsernames[client]
	clientUserMu.RUnlock()

	if username == "" {
		username = "Player"
	}

	// 1.12.2 Serverbound Packet IDs
	switch packetID {
	case 0x01: // Tab-Complete
		Play.HandleTabComplete(client, packetData)
	case 0x02: // Chat Message
		Play.HandleChatMessage(client, packetData, username)
	case 0x03: // Client Status
		Play.HandleClientStatus(client, packetData)
	case 0x04: // Client Settings
		Play.HandleClientSettings(client, packetData)
	case 0x06: // Animation (Serverbound - 玩家挥臂)
		Play.HandleAnimation(client, packetData, username)
	case 0x07: // Player Digging
		Play.HandlePlayerDigging(client, packetData)
	case 0x08:
		Play.HandlePlayerBlockPlacement(client, packetData)
	case 0x09:
		Play.HandleClientCapabilities(client, packetData)
	case 0x0B: // Keep Alive (Serverbound)
		Play.HandleKeepAlive(client, packetData)
	case 0x0D: // Player Position (Serverbound)
		Play.HandlePlayerPosition(client, packetData)
	case 0x0F: // Player Rotation (Serverbound)
		Play.HandlePlayerLook(client, packetData)
	case 0x0E: // Player Position and Rotation (Serverbound)
		Play.HandlePlayerPositionAndLook(client, packetData)
	case 0x14, 0x15:
		Play.HandleEntityAction(client, packetData)
	case 0x16:
		Play.HandleVehicleMove(client, packetData)
	case 0x17:
		Play.HandleCreativeInventoryAction(client, packetData)
	case 0x1A:
		Play.HandleClickWindow(client, packetData)
	case 0x1B:
		Play.HandleCloseWindow(client, packetData)
	case 0x1D:
		Play.HandleTransaction(client, packetData)
	}
}
