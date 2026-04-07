// Package Tick 掉落物实体tick处理
package Tick

import (
	"RootreeMC/entity"
	"fmt"
)

// processItemEntities 处理所有掉落物实体
func processItemEntities() {
	// 示例：每100tick创建一个测试掉落物
	if GetWorldAge()%100 == 0 {
		// 创建测试掉落物（钻石）
		eid := entity.GlobalEntityManager.CreateItemEntity(
			264, // ItemID
			1,   // Count
			nil, // 无NBT
			8.5, 65.0, 8.5, // Pos
			0, 0.2, 0,      // Velocity (向上抛出)
		)
		
		if eid > 0 {
			fmt.Printf("[Tick] 创建测试掉落物: EID=%d\n", eid)
			
			// 广播给所有玩家 (这里可能需要更完善的广播逻辑)
			itemEnt := entity.GlobalEntityManager.GetItemEntity(eid)
			if itemEnt != nil {
				// 获取所有在线玩家并发送生成包
				// 注意：这里可能需要引入 player 包，但为了避免循环依赖，建议在 Packet Manager 中处理
			}
		}
	}
}

// SpawnItemEntity 生成掉落物实体（供外部调用）
func SpawnItemEntity(itemID int32, count int32, nbtData []byte, x, y, z float64, vX, vY, vZ float64) int32 {
	return entity.GlobalEntityManager.CreateItemEntity(itemID, count, nbtData, x, y, z, vX, vY, vZ)
}

// CreateItemDrop 创建物品掉落（简化接口）
func CreateItemDrop(itemID int32, count byte, x, y, z float64) {
	entity.GlobalEntityManager.CreateItemEntity(int32(itemID), int32(count), nil, x, y, z, 0, 0, 0)
}
