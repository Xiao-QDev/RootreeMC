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

const spawnSafetySearchRadius int32 = 96

// WorldManager 管理所有加载的区块
type WorldManager struct {
	chunks     map[[2]int32]*Chunk // key: [chunkX, chunkZ]
	dirtyChunks map[[2]int32]bool
	saveDir    string
	spawnX     int32
	spawnY     int32
	spawnZ     int32
	mu         sync.RWMutex
}

// GlobalWorld 全局世界管理器实例
var GlobalWorld *WorldManager

func init() {
	GlobalWorld = NewWorldManager()
	if err := GlobalWorld.LoadBlockTickState(); err != nil {
		slog.Warn("[World] 读取方块Tick存档失败", "err", err)
	}
}

// NewWorldManager 创建新的世界管理器
func NewWorldManager() *WorldManager {
	dir := defaultChunkSaveDir()
	if err := ensureDir(dir); err != nil {
		slog.Warn("[World] 创建区块存档目录失败", "dir", dir, "err", err)
	}

	wm := &WorldManager{
		chunks:      make(map[[2]int32]*Chunk),
		dirtyChunks: make(map[[2]int32]bool),
		saveDir:     dir,
	}

	if err := wm.ConvertAnvilToLinearV2IfNeeded(); err != nil {
		slog.Warn("[World] ANVIL 转换失败", "err", err)
	}
	wm.RecalculateSpawnPoint()

	return wm
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

	// 先尝试从存档加载
	loaded, err := wm.loadChunkFromDisk(chunkX, chunkZ)
	if err != nil {
		slog.Warn("[World] 读取区块存档失败，回退到生成", "x", chunkX, "z", chunkZ, "err", err)
	}
	if loaded != nil {
		wm.chunks[key] = loaded
		slog.Info("[World] 从存档加载区块", "x", chunkX, "z", chunkZ)
		return loaded
	}

	// 不存在存档，生成新区块
	chunk = NewChunk(chunkX, chunkZ)
	chunk.GenerateChunk()
	wm.chunks[key] = chunk
	wm.dirtyChunks[key] = true // 新区块标记为脏，后续落盘

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

// RecalculateSpawnPoint 重新计算世界出生点（原版风格搜索）
func (wm *WorldManager) RecalculateSpawnPoint() {
	baseX, baseY, baseZ := FindVanillaSpawnPoint()
	x, y, z, ok := wm.findNearestSafeSpawn(baseX, baseZ, spawnSafetySearchRadius)
	if !ok {
		x, y, z = baseX, baseY, baseZ
	}

	wm.mu.Lock()
	wm.spawnX = x
	wm.spawnY = y
	wm.spawnZ = z
	wm.mu.Unlock()

	slog.Info("[World] 已更新出生点", "x", x, "y", y, "z", z)
}

// GetSpawnPoint 返回当前世界出生点
func (wm *WorldManager) GetSpawnPoint() (int32, int32, int32) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.spawnX, wm.spawnY, wm.spawnZ
}

func worldToChunkAndBlock(world int32) (int32, int32) {
	chunk := world / 16
	block := world % 16
	if block < 0 {
		block += 16
		chunk--
	}
	return chunk, block
}

func isSpawnPassableID(id uint16) bool {
	switch id {
	case 0, 6, 31, 32, 37, 38, 39, 40, 78, 175:
		return true
	default:
		return false
	}
}

func isSpawnGroundID(id uint16) bool {
	switch id {
	case 0, 6, 8, 9, 10, 11, 18, 31, 32, 37, 38, 39, 40, 50, 51, 65, 75, 76, 78, 175:
		return false
	default:
		return true
	}
}

func (wm *WorldManager) getBlockAutoLoad(worldX, worldY, worldZ int32) uint16 {
	if worldY < 0 || worldY >= 256 {
		return 0
	}

	chunkX, blockX := worldToChunkAndBlock(worldX)
	chunkZ, blockZ := worldToChunkAndBlock(worldZ)
	chunk := wm.GetOrCreateChunk(chunkX, chunkZ)
	if chunk == nil {
		return 0
	}
	return chunk.Blocks[blockX][worldY][blockZ]
}

// IsSafeSpawnAt 检查给定脚部坐标是否可作为安全出生点。
func (wm *WorldManager) IsSafeSpawnAt(worldX, worldY, worldZ int32) bool {
	if worldY < 1 || worldY >= 255 {
		return false
	}

	groundID := GetID(wm.getBlockAutoLoad(worldX, worldY-1, worldZ))
	feetID := GetID(wm.getBlockAutoLoad(worldX, worldY, worldZ))
	headID := GetID(wm.getBlockAutoLoad(worldX, worldY+1, worldZ))
	return isSpawnGroundID(groundID) && isSpawnPassableID(feetID) && isSpawnPassableID(headID)
}

func (wm *WorldManager) findSafeYAt(worldX, worldZ int32) (int32, bool) {
	// 优先从地表附近向上下搜索，避免整列全扫。
	height := int32(GetHeight(int(worldX), int(worldZ)) + 1)
	if height < 1 {
		height = 1
	}
	if height > 254 {
		height = 254
	}

	for delta := int32(0); delta <= 32; delta++ {
		up := height + delta
		if up <= 254 && wm.IsSafeSpawnAt(worldX, up, worldZ) {
			return up, true
		}
		down := height - delta
		if down >= 1 && wm.IsSafeSpawnAt(worldX, down, worldZ) {
			return down, true
		}
	}

	for y := int32(254); y >= 1; y-- {
		if wm.IsSafeSpawnAt(worldX, y, worldZ) {
			return y, true
		}
	}
	return 0, false
}

func (wm *WorldManager) findNearestSafeSpawn(startX, startZ, maxRadius int32) (int32, int32, int32, bool) {
	if y, ok := wm.findSafeYAt(startX, startZ); ok {
		return startX, y, startZ, true
	}

	for r := int32(1); r <= maxRadius; r++ {
		minX, maxX := startX-r, startX+r
		minZ, maxZ := startZ-r, startZ+r

		for x := minX; x <= maxX; x++ {
			if y, ok := wm.findSafeYAt(x, minZ); ok {
				return x, y, minZ, true
			}
			if minZ != maxZ {
				if y, ok := wm.findSafeYAt(x, maxZ); ok {
					return x, y, maxZ, true
				}
			}
		}

		for z := minZ + 1; z <= maxZ-1; z++ {
			if y, ok := wm.findSafeYAt(minX, z); ok {
				return minX, y, z, true
			}
			if minX != maxX {
				if y, ok := wm.findSafeYAt(maxX, z); ok {
					return maxX, y, z, true
				}
			}
		}
	}

	return 0, 0, 0, false
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

	oldState := chunk.Blocks[blockX][worldY][blockZ]
	if oldState == blockID {
		return false
	}

	chunk.Blocks[blockX][worldY][blockZ] = blockID

	wm.mu.Lock()
	wm.dirtyChunks[[2]int32{chunkX, chunkZ}] = true
	wm.mu.Unlock()

	if GlobalWorldSimulation != nil {
		GlobalWorldSimulation.OnBlockChanged(worldX, worldY, worldZ, blockID)
	}

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
