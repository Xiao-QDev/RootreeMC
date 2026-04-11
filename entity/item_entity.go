// Package entity 掉落物实体
package entity

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/inventory"
	"bytes"
	"encoding/binary"
	"fmt"
)

// ItemEntity 掉落物实体
type ItemEntity struct {
	Entity
	Item        inventory.ItemStack // 物品
	Age         int32               // 存在时间（tick）
	PickupDelay int32               // 拾取延迟（tick）
	Health      int16               // 生命值（用于摧毁）
	Owner       string              // 所有者（防止重复拾取）
	VelocityX   float64             // X速度
	VelocityY   float64             // Y速度
	VelocityZ   float64             // Z速度
}

// CreateItemEntity 创建掉落物实体
func (em *EntityManager) CreateItemEntity(itemID int32, count int32, nbtData []byte, x, y, z float64, vX, vY, vZ float64) int32 {
	em.mu.Lock()
	defer em.mu.Unlock()

	eid := em.nextEID
	em.nextEID++

	itemStack := inventory.ItemStack{
		ItemID:   itemID,
		Count:    byte(count),
		Damage:   0,
		HasNBT:   len(nbtData) > 0,
		NBTData:  nbtData,
	}

	itemEntity := &ItemEntity{
		Entity: Entity{
			EID:      eid,
			Type:     EntityTypeItem,
			X:        x,
			Y:        y,
			Z:        z,
			Yaw:      0,
			Pitch:    0,
			OnGround: false,
			Metadata: make(map[byte]EntityMetadata),
		},
		Item:        itemStack,
		Age:         0,
		PickupDelay: 40,
		Health:      5,
		VelocityX:   vX,
		VelocityY:   vY,
		VelocityZ:   vZ,
	}

	// 1.12.2: Index 6 (Item) = Slot Data
	itemEntity.Metadata[6] = EntityMetadata{
		Index: 6,
		Type:  5,
		Value: buildItemSlotData(itemStack),
	}

	em.entities[eid] = &itemEntity.Entity
	em.items[eid] = itemEntity
	fmt.Printf("[ItemEntity] 创建掉落物实体: EID=%d, ItemID=%d, Count=%d, Pos=(%.2f, %.2f, %.2f)\n", eid, itemID, count, x, y, z)

	return eid
}

// GetItemEntity 获取掉落物实体
func (em *EntityManager) GetItemEntity(eid int32) *ItemEntity {
	em.mu.RLock()
	defer em.mu.RUnlock()

	itemEntity, ok := em.items[eid]
	if !ok {
		return nil
	}
	return itemEntity
}

// BuildSpawnItemEntity 生成掉落物实体包 (0x00)
func BuildSpawnItemEntity(itemEntity *ItemEntity) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x00) // Packet ID: Spawn Object
	Network.WriteVarint(buf, itemEntity.EID)
	Protocol.WriteUUID(buf, make([]byte, 16))
	buf.WriteByte(2) // Type: Item (2)
	Protocol.WriteDouble(buf, itemEntity.X)
	Protocol.WriteDouble(buf, itemEntity.Y)
	Protocol.WriteDouble(buf, itemEntity.Z)
	buf.WriteByte(0)                              // Pitch
	buf.WriteByte(0)                              // Yaw
	binary.Write(buf, binary.BigEndian, int32(1)) // Data: 1 (Has Item)
	Protocol.WriteShort(buf, int16(itemEntity.VelocityX*8000))
	Protocol.WriteShort(buf, int16(itemEntity.VelocityY*8000))
	Protocol.WriteShort(buf, int16(itemEntity.VelocityZ*8000))
	return Protocol.AddLengthPrefix(buf)
}

// buildItemSlotData 构建物品槽数据
func buildItemSlotData(item inventory.ItemStack) []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, int16(item.ItemID))
	buf.WriteByte(item.Count)
	binary.Write(buf, binary.BigEndian, int16(item.Damage))
	buf.WriteByte(0) // No NBT
	return buf.Bytes()
}
