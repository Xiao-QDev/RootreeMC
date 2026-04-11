// Package Tick 掉落物实体tick处理
package Tick

import (
	"RootreeMC/entity"
)

// processItemEntities 处理所有掉落物实体
func processItemEntities() {
	// 禁用测试掉落物自动生成；仅保留真实游戏行为产生的掉落物。
}

// SpawnItemEntity 生成掉落物实体（供外部调用）
func SpawnItemEntity(itemID int32, count int32, nbtData []byte, x, y, z float64, vX, vY, vZ float64) int32 {
	return entity.GlobalEntityManager.CreateItemEntity(itemID, count, nbtData, x, y, z, vX, vY, vZ)
}

// CreateItemDrop 创建物品掉落（简化接口）
func CreateItemDrop(itemID int32, count byte, x, y, z float64) {
	entity.GlobalEntityManager.CreateItemEntity(int32(itemID), int32(count), nil, x, y, z, 0, 0, 0)
}
