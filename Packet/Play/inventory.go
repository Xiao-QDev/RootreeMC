// Package Play 物品和窗口相关包处理
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/entity"
	"RootreeMC/player"
	"bytes"
	"encoding/binary"
	"fmt"
)

// ItemStack 物品栈
type ItemStack struct {
	ID    int32
	Count int16
	NBT   []byte
}

// HandleCreativeInventoryAction 处理创造模式物品栏操作
// Packet ID: 0x17 (1.12.2)
func HandleCreativeInventoryAction(client *Network.Network, data []byte) {
	if len(data) < 3 {
		return
	}

	reader := bytes.NewReader(data)
	slot, _ := Network.ReadVarint(reader)

	var item ItemStack

	if len(data) > 2 {
		itemCount, _ := Network.ReadVarint(reader)
		item.Count = int16(itemCount)

		if itemCount > 0 {
			item.ID, _ = Network.ReadVarint(reader)
			// NBT 数据
			if reader.Len() > 0 {
				nbtLen, _ := Network.ReadVarint(reader)
				if nbtLen > 0 {
					nbtData := make([]byte, nbtLen)
					reader.Read(nbtData)
					item.NBT = nbtData
				}
			}
		}
	}

	fmt.Printf("[Play] 创造模式物品操作: slot=%d, item=%d:%d\n", slot, item.ID, item.Count)

	// 处理丢弃物品 (slot = -999 表示丢弃物品)
	if slot == -999 && item.Count > 0 {
		handleDropItem(client, &item)
	}
}

// handleDropItem 处理玩家丢弃物品
func handleDropItem(client *Network.Network, item *ItemStack) {
	p := player.GlobalPlayerManager.GetPlayerByClient(client)
	if p == nil {
		return
	}

	// 获取玩家位置
	posX := p.PlayerEntity.X
	posY := p.PlayerEntity.Y
	posZ := p.PlayerEntity.Z

	// 在玩家位置生成掉落物
	eid := entity.GlobalEntityManager.CreateItemEntity(
		int32(item.ID),
		int32(item.Count),
		item.NBT,
		posX, posY, posZ,
		0, 0, 0, // 初始速度为0
	)

	if eid > 0 {
		fmt.Printf("[DropItem] 玩家 %s 丢弃物品: ID=%d, Count=%d, EID=%d\n", 
			p.Username, item.ID, item.Count, eid)

// 广播给所有玩家
	itemEntity := entity.GlobalEntityManager.GetItemEntity(eid)
	if itemEntity != nil {
		fmt.Printf("[DropItem] 获取到掉落物实体: EID=%d, ItemID=%d, Count=%d\n", 
			itemEntity.EID, itemEntity.Item.ItemID, itemEntity.Item.Count)
		
		// 1. 发送 Spawn Item Entity 包
		spawnItemPkt := entity.BuildSpawnItemEntity(itemEntity)
		fmt.Printf("[DropItem] 生成SpawnItem数据包, 长度=%d字节\n", len(spawnItemPkt))
		
		broadcastToAllPlayers(spawnItemPkt)
		
		// 2. 发送 Entity Metadata 包（设置物品堆叠）
		metaPkt := BuildItemEntityMetadata(itemEntity)
		fmt.Printf("[DropItem] 生成Metadata数据包, 长度=%d字节\n", len(metaPkt))
		
		broadcastToAllPlayers(metaPkt)
		fmt.Printf("[DropItem] 广播完成, 在线玩家数=%d\n", len(player.GlobalPlayerManager.GetAllOnlinePlayers()))
	} else {
		fmt.Printf("[DropItem] 警告: 无法获取掉落物实体 EID=%d\n", eid)
	}
	}
}

// broadcastToAllPlayers 广播数据包给所有在线玩家
func broadcastToAllPlayers(packet []byte) {
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	for _, p := range allPlayers {
		p.Client.Send(packet)
	}
}

// BuildItemEntityMetadata 构建物品实体的元数据包
func BuildItemEntityMetadata(itemEntity *entity.ItemEntity) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x39) // Packet ID: Entity Metadata (1.12.2)
	Network.WriteVarint(buf, itemEntity.EID)
	
	// 构建元数据: Index 6, Type 5 (Slot)
	buf.WriteByte(6) // Index
	buf.WriteByte(5) // Type (Slot)
	
	// Slot 数据格式: Item ID (VarInt) + Count (Byte) + NBT (Tag)
	Network.WriteVarint(buf, itemEntity.Item.ItemID)
	buf.WriteByte(byte(itemEntity.Item.Count))
	
	// NBT (空标签表示无NBT)
	buf.WriteByte(0) // Tag ID: 0 (TAG_End)
	
	buf.WriteByte(0xFF) // Metadata结束标记
	
	return Protocol.AddLengthPrefix(buf)
}

// HandleTransaction 处理交易确认
// Packet ID: 0x1D (1.12.2)
func HandleTransaction(client *Network.Network, data []byte) {
	if len(data) < 10 {
		return
	}

	reader := bytes.NewReader(data)
	windowID, _ := Network.ReadVarint(reader)
	actionNumber := binary.BigEndian.Uint16(data[1:3])
	accepted := data[3] != 0

	fmt.Printf("[Play] 交易确认: windowID=%d, action=%d, accepted=%v\n", windowID, actionNumber, accepted)
}

// HandleTabComplete 处理 Tab 补全请求
// Packet ID: 0x05 (1.12.2)
func HandleTabComplete(client *Network.Network, data []byte) {
	if len(data) < 1 {
		return
	}

	reader := bytes.NewReader(data)
	text, _ := Protocol.ReadString(reader)

	// TODO: 实现命令补全逻辑
	fmt.Printf("[Play] Tab 补全请求: %s\n", text)
}
