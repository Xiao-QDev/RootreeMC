// Package world 世界管理器
package world

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/binary"
	"log/slog"
	"sync"
)

// WorldManager 管理所有加载的区块
type WorldManager struct {
	chunks map[[2]int32]*Chunk // key: [chunkX, chunkZ]
	mu     sync.RWMutex
}

// GlobalWorld 全局世界管理器实例
var GlobalWorld *WorldManager

func init() {
	GlobalWorld = NewWorldManager()
}

// NewWorldManager 创建新的世界管理器
func NewWorldManager() *WorldManager {
	return &WorldManager{
		chunks: make(map[[2]int32]*Chunk),
	}
}

// GetOrCreateChunk 获取区块，不存在则创建
func (wm *WorldManager) GetOrCreateChunk(chunkX, chunkZ int32) *Chunk {
	key := [2]int32{chunkX, chunkZ}

	wm.mu.RLock()
	chunk, exists := wm.chunks[key]
	wm.mu.RUnlock()

	if exists {
		return chunk
	}

	// 不存在，创建新区块
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// 双重检查
	if chunk, exists = wm.chunks[key]; exists {
		return chunk
	}

	// 从存档加载或生成新区块
	chunk = NewChunk(chunkX, chunkZ)
	chunk.GenerateChunk() // 使用地形生成
	wm.chunks[key] = chunk

	slog.Info("[World] 加载/生成区块", "x", chunkX, "z", chunkZ)
	return chunk
}

// GetChunk 获取区块，不存在返回 nil
func (wm *WorldManager) GetChunk(chunkX, chunkZ int32) *Chunk {
	key := [2]int32{chunkX, chunkZ}

	wm.mu.RLock()
	defer wm.mu.RUnlock()

	return wm.chunks[key]
}

// SetBlock 设置方块
func (wm *WorldManager) SetBlock(worldX, worldY, worldZ int32, blockID uint16) bool {
	if worldY < 0 || worldY >= 256 {
		return false
	}

	chunkX := worldX / 16
	chunkZ := worldZ / 16
	blockX := worldX % 16
	blockZ := worldZ % 16

	// 处理负数坐标
	if blockX < 0 {
		blockX += 16
		chunkX--
	}
	if blockZ < 0 {
		blockZ += 16
		chunkZ--
	}

	chunk := wm.GetChunk(chunkX, chunkZ)
	if chunk == nil {
		return false
	}

	chunk.Blocks[blockX][worldY][blockZ] = blockID
	return true
}

// GetBlock 获取方块
func (wm *WorldManager) GetBlock(worldX, worldY, worldZ int32) uint16 {
	if worldY < 0 || worldY >= 256 {
		return 0 // 空气
	}

	chunkX := worldX / 16
	chunkZ := worldZ / 16
	blockX := worldX % 16
	blockZ := worldZ % 16

	// 处理负数坐标
	if blockX < 0 {
		blockX += 16
		chunkX--
	}
	if blockZ < 0 {
		blockZ += 16
		chunkZ--
	}

	chunk := wm.GetChunk(chunkX, chunkZ)
	if chunk == nil {
		return 0 // 空气
	}

	return chunk.Blocks[blockX][worldY][blockZ]
}

// BuildBlockChange 构建方块更新包 (0x0B)
func BuildBlockChange(worldX, worldY, worldZ int32, blockID uint16) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x0B) // Block Change Packet ID

	// Position: 8 bytes, encoded as (x << 38) | (z << 12) | y
	pos := (uint64(worldX&0x3FFFFFF) << 38) | (uint64(worldZ&0x3FFFFFF) << 12) | uint64(worldY&0xFFF)
	binary.Write(buf, binary.BigEndian, pos)

	Network.WriteVarint(buf, int32(blockID))

	// 包装长度前缀
	data := buf.Bytes()
	result := &bytes.Buffer{}
	Network.WriteVarint(result, int32(len(data)))
	result.Write(data)

	return result.Bytes()
}

// BuildMultiBlockChange 构建多方块更新包 (0x10)
// 用于批量更新区块内的方块
func BuildMultiBlockChange(chunkX, chunkZ int32, changes []BlockChange) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x10) // Multi Block Change Packet ID
	Protocol.WriteInt(buf, chunkX)
	Protocol.WriteInt(buf, chunkZ)
	Network.WriteVarint(buf, int32(len(changes)))

	for _, change := range changes {
		// 位置: 12 bits (x) | 12 bits (z) | 8 bits (y)
		pos := ((change.X & 0xF) << 12) | ((change.Z & 0xF) << 8) | (change.Y & 0xFF)
		// WriteShort: 2 bytes BigEndian
		binary.Write(buf, binary.BigEndian, int16(pos))
		Network.WriteVarint(buf, int32(change.BlockID))
	}

	// 包装长度前缀
	data := buf.Bytes()
	result := &bytes.Buffer{}
	Network.WriteVarint(result, int32(len(data)))
	result.Write(data)

	return result.Bytes()
}

// BlockChange 单个方块变更
type BlockChange struct {
	X, Y, Z int32
	BlockID uint16
}
