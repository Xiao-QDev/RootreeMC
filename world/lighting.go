// Package world 世界光照系统
package world

import (
	"sync"
)

// LightLevel 光照等级 (0-15)
type LightLevel byte

const (
	MinLightLevel LightLevel = 0  // 完全黑暗
	MaxLightLevel LightLevel = 15 // 最大亮度
)

// LightingEngine 光照引擎
type LightingEngine struct {
	mu          sync.RWMutex
	worldLight  map[string]LightLevel // 世界光照 (方块位置 -> 光照等级)
	skyLight    map[string]LightLevel // 天空光照
	lightQueue  []LightUpdate         // 光照更新队列
}

// LightUpdate 光照更新
type LightUpdate struct {
	ChunkX int32
	ChunkZ int32
	X      int32 // 方块坐标
	Y      int32
	Z      int32
	Level  LightLevel
	Source LightSource
}

// LightSource 光源类型
type LightSource int

const (
	LightSourceBlock LightSource = iota // 方块发光（火把、萤石等）
	LightSourceSky                       // 天空光
	LightSourceSun                       // 太阳光
)

// GlobalLightingEngine 全局光照引擎
var GlobalLightingEngine *LightingEngine

// BroadcastCallback 广播回调函数类型 (用于避免循环导入)
type BroadcastCallback func(pkt []byte)

// broadcastCallback 全局广播回调
var broadcastCallback BroadcastCallback

// RegisterBroadcastCallback 注册广播回调函数
func RegisterBroadcastCallback(callback BroadcastCallback) {
	broadcastCallback = callback
}

func init() {
	GlobalLightingEngine = NewLightingEngine()
}

// NewLightingEngine 创建新的光照引擎
func NewLightingEngine() *LightingEngine {
	return &LightingEngine{
		worldLight: make(map[string]LightLevel),
		skyLight:   make(map[string]LightLevel),
	}
}

// CalculateLightAt 计算指定位置的光照等级
func (le *LightingEngine) CalculateLightAt(x, y, z int32) LightLevel {
	le.mu.RLock()
	defer le.mu.RUnlock()

	// 天空光照
	skyKey := le.getLightKey(x, y, z)
	skyLevel, hasSky := le.skyLight[skyKey]

	// 方块光照
	blockKey := le.getLightKey(x, y, z)
	blockLevel, hasBlock := le.worldLight[blockKey]

	// 返回两者中较高的光照等级
	if !hasSky && !hasBlock {
		return MinLightLevel
	}

	if hasSky && hasBlock {
		if skyLevel > blockLevel {
			return skyLevel
		}
		return blockLevel
	}

	if hasSky {
		return skyLevel
	}

	return blockLevel
}

// SetBlockLight 设置方块光照
func (le *LightingEngine) SetBlockLight(x, y, z int32, level LightLevel) {
	le.mu.Lock()
	defer le.mu.Unlock()

	key := le.getLightKey(x, y, z)
	le.worldLight[key] = level

	// 添加到更新队列
	le.lightQueue = append(le.lightQueue, LightUpdate{
		ChunkX: x >> 4,
		ChunkZ: z >> 4,
		X:      x,
		Y:      y,
		Z:      z,
		Level:  level,
		Source: LightSourceBlock,
	})
}

// SetSkyLight 设置天空光照
func (le *LightingEngine) SetSkyLight(x, y, z int32, level LightLevel) {
	le.mu.Lock()
	defer le.mu.Unlock()

	key := le.getLightKey(x, y, z)
	le.skyLight[key] = level

	// 添加到更新队列
	le.lightQueue = append(le.lightQueue, LightUpdate{
		ChunkX: x >> 4,
		ChunkZ: z >> 4,
		X:      x,
		Y:      y,
		Z:      z,
		Level:  level,
		Source: LightSourceSky,
	})
}

// UpdateChunkLighting 更新整个区块的光照
func (le *LightingEngine) UpdateChunkLighting(chunkX, chunkZ int32) {
	le.mu.Lock()
	defer le.mu.Unlock()

	// 简化的光照计算：根据高度设置天空光照
	for y := int32(0); y < 256; y++ {
		for z := int32(0); z < 16; z++ {
			for x := int32(0); x < 16; x++ {
				worldX := chunkX*16 + x
				worldZ := chunkZ*16 + z

				// 简单规则：y > 64 的地方有天空光
				if y > 64 {
					le.SetSkyLight(worldX, y, worldZ, MaxLightLevel)
				} else {
					le.SetSkyLight(worldX, y, worldZ, MinLightLevel)
				}
			}
		}
	}
}

// ProcessLightUpdates 处理光照更新队列
func (le *LightingEngine) ProcessLightUpdates() {
	le.mu.Lock()
	if len(le.lightQueue) == 0 {
		le.mu.Unlock()
		return
	}

	// 按区块分组更新
	updatesByChunk := make(map[[2]int32][]LightUpdate)
	for _, update := range le.lightQueue {
		key := [2]int32{update.ChunkX, update.ChunkZ}
		updatesByChunk[key] = append(updatesByChunk[key], update)
	}

	// 清空队列
	le.lightQueue = le.lightQueue[:0]
	le.mu.Unlock()

	// 1.12.2 不支持独立光照包，光照通常通过重发区块或区块段来更新。
	// 这里可以添加逻辑来标记区块需要重新同步，或者在 1.12.2 中忽略动态光照更新。
}

// buildSectionLightData 构建区块段的光照数据
func (le *LightingEngine) buildSectionLightData(chunkX, chunkZ int32, updates []LightUpdate) (skyLight, blockLight []byte) {
	// 每个段: 16x16x16 = 4096个方块
	// 每个方块: 4位光照数据
	// 每段数据: 4096 / 2 = 2048字节
	
	sectionCount := int32(16)
	bytesPerSection := 2048
	totalBytes := int(sectionCount) * bytesPerSection

	skyLight = make([]byte, totalBytes)
	blockLight = make([]byte, totalBytes)

	// 填充基础光照
	for sectionY := int32(0); sectionY < sectionCount; sectionY++ {
		baseIndex := int(sectionY) * bytesPerSection
		
		// 简化的光照计算
		for i := 0; i < bytesPerSection; i++ {
			// 高段(y>8)有天空光
			if sectionY > 8 {
				skyLight[baseIndex+i] = 0xFF // 全亮
			} else {
				skyLight[baseIndex+i] = 0x00 // 全暗
			}
			blockLight[baseIndex+i] = 0x00 // 默认无方块光
		}
	}

	// 应用更新
	for _, update := range updates {
		sectionY := update.Y >> 4 // Y / 16
		if sectionY < 0 || sectionY >= sectionCount {
			continue
		}

		// 计算在段内的索引
		relX := update.X & 0x0F // X % 16
		relY := update.Y & 0x0F // Y % 16
		relZ := update.Z & 0x0F // Z % 16

		index := int(sectionY)*bytesPerSection + (int(relY)*256 + int(relZ)*16 + int(relX))/2
		nibble := (int(relY)*256 + int(relZ)*16 + int(relX)) % 2

		if update.Source == LightSourceSky {
			if nibble == 0 {
				skyLight[index] = (skyLight[index] & 0x0F) | (byte(update.Level) << 4)
			} else {
				skyLight[index] = (skyLight[index] & 0xF0) | byte(update.Level)
			}
		} else {
			if nibble == 0 {
				blockLight[index] = (blockLight[index] & 0x0F) | (byte(update.Level) << 4)
			} else {
				blockLight[index] = (blockLight[index] & 0xF0) | byte(update.Level)
			}
		}
	}

	return skyLight, blockLight
}

// getLightKey 生成光照键
func (le *LightingEngine) getLightKey(x, y, z int32) string {
	// 使用64位整数编码坐标 (类似Chunk Position的编码方式)
	return string([]byte{
		byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x),
		byte(y >> 24), byte(y >> 16), byte(y >> 8), byte(y),
		byte(z >> 24), byte(z >> 16), byte(z >> 8), byte(z),
	})
}

// buildUpdateLightPacket 修正：1.12.2 不支持独立光照更新包
func (le *LightingEngine) buildUpdateLightPacket(chunkX, chunkZ int32, skyLight, blockLight []byte) []byte {
	return nil
}

// BuildSimpleLightUpdate 修正：1.12.2 不需要独立光照包，光照已包含在 Chunk Data (0x20) 中
func BuildSimpleLightUpdate(chunkX, chunkZ int32) []byte {
	return nil
}

// buildSimpleLightUpdate 修正：1.12.2 不需要独立光照包
func (le *LightingEngine) buildSimpleLightUpdate(chunkX, chunkZ int32) []byte {
	return nil
}

// buildChunkLightData 构建简化的区块光照数据
func (le *LightingEngine) buildChunkLightData(sectionCount int32) (skyLight, blockLight []byte) {
	// 每个段: 16x16x16 = 4096个方块
	// 每个方块: 4位光照数据 (0-15)
	// 每段数据大小: 4096 / 2 = 2048字节 (每个字节存储2个方块的光照)

	bytesPerSection := 2048
	totalBytes := int(sectionCount) * bytesPerSection

	// 天空光照：假设全亮 (15)
	skyLight = make([]byte, totalBytes)
	for i := range skyLight {
		skyLight[i] = 0xFF // 每个字节2个方块，都是15 (0xF)
	}

	// 方块光照：假设全暗 (0)
	blockLight = make([]byte, totalBytes)
	for i := range blockLight {
		blockLight[i] = 0x00
	}

	return skyLight, blockLight
}

// CalculateNaturalLight 计算自然光照（用于区块生成）
func (le *LightingEngine) CalculateNaturalLight(chunkX, chunkZ int32) {
	le.mu.Lock()
	defer le.mu.Unlock()

	// 简化的自然光照计算：根据高度设置
	for y := int32(0); y < 256; y++ {
		for z := int32(0); z < 16; z++ {
			for x := int32(0); x < 16; x++ {
				worldX := chunkX*16 + x
				worldY := y
				worldZ := chunkZ*16 + z

				// 规则1: y >= 64 的地方有天空光
				if worldY >= 64 {
					le.skyLight[le.getLightKey(worldX, worldY, worldZ)] = MaxLightLevel
				} else {
					// 地下：线性衰减
					level := MaxLightLevel - LightLevel((64-worldY)/4)
					if level < MinLightLevel {
						level = MinLightLevel
					}
					le.skyLight[le.getLightKey(worldX, worldY, worldZ)] = level
				}

				// 规则2: 发光方块（简化：空气=0，其他=15）
				blockID := GlobalWorld.GetBlock(worldX, worldY, worldZ)
				if blockID != 0 {
					// 假设所有方块都发光（实际应该根据方块类型）
					if blockID == 50 || blockID == 89 || blockID == 124 { // 火把、萤石、红石灯
						le.worldLight[le.getLightKey(worldX, worldY, worldZ)] = MaxLightLevel
					}
				}
			}
		}
	}
}

// UpdateBlockLight 更新单个方块的光照（用于放置/破坏方块）
func (le *LightingEngine) UpdateBlockLight(x, y, z int32, blockID uint16) {
	// 检查是否是发光方块
	lightLevel := getBlockLightLevel(blockID)
	
	if lightLevel > 0 {
		le.SetBlockLight(x, y, z, LightLevel(lightLevel))
	} else {
		le.SetBlockLight(x, y, z, MinLightLevel)
	}
}

// getBlockLightLevel 获取方块的发光等级
func getBlockLightLevel(blockID uint16) byte {
	switch blockID {
	case 50:  // 火把
		return 14
	case 51:  // 火
		return 15
	case 89:  // 萤石
		return 15
	case 124: // 红石灯（亮）
		return 15
	case 169: // 海晶灯
		return 15
	case 138: // 信标
		return 15
	default:
		return 0
	}
}
