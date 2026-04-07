// Package world 世界数据
package world

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/binary"
	"fmt"
)

// BlockState 1.12.2 方块状态 (ID << 4 | Metadata)
type BlockState uint16

// ToState 将 ID 和 Metadata 转换为方块状态
func ToState(id uint16, data uint8) uint16 {
	return (id << 4) | uint16(data&0x0F)
}

// GetID 从状态获取 ID
func GetID(state uint16) uint16 {
	return state >> 4
}

// GetData 从状态获取 Metadata
func GetData(state uint16) uint8 {
	return uint8(state & 0x0F)
}

// Chunk 区块数据 (16x16x256)
type Chunk struct {
	X, Z   int32
	Blocks [16][256][16]uint16 // [x][y][z] 方块状态 (ID << 4 | Data)
}

// NewChunk 创建空区块
func NewChunk(x, z int32) *Chunk {
	return &Chunk{X: x, Z: z}
}

// BuildMapChunk 构建 1.12.2 区块数据包 (从 Packet/Play 移过来以避免循环导入)
func BuildMapChunk(chunk *Chunk) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x20) // Packet ID: Chunk Data (1.12.2)

	Protocol.WriteInt(buf, chunk.X)
	Protocol.WriteInt(buf, chunk.Z)
	Protocol.WriteBoolean(buf, true) // Full Chunk=true

	// 计算 mask
	mask := int32(0)
	for i := 0; i < 16; i++ {
		if sectionHasBlocks(chunk, i) {
			mask |= (1 << uint(i))
		}
	}
	Network.WriteVarint(buf, mask)

	// 构建区块数据（不压缩）
	data := &bytes.Buffer{}
	for i := 0; i < 16; i++ {
		if mask&(1<<uint(i)) != 0 {
			writeSection(data, chunk, i)
		}
	}

	// 生物群系 (256 字节)
	biomes := make([]byte, 256)
	for i := range biomes {
		biomes[i] = 1 // Plains
	}
	data.Write(biomes)

	// 直接发送原始数据（不压缩）
	dataBytes := data.Bytes()
	Network.WriteVarint(buf, int32(len(dataBytes)))
	buf.Write(dataBytes)

	Network.WriteVarint(buf, 0) // Block Entities

	return wrapPacket(buf)
}

func sectionHasBlocks(chunk *Chunk, sectionY int) bool {
	baseY := sectionY * 16
	for y := 0; y < 16; y++ {
		worldY := baseY + y
		if worldY >= 256 {
			break
		}
		for x := 0; x < 16; x++ {
			for z := 0; z < 16; z++ {
				if chunk.Blocks[x][worldY][z] != 0 {
					return true
				}
			}
		}
	}
	return false
}

// writeSection 数据编码 (BitsPerBlock=4, 本地调色板)
func writeSection(data *bytes.Buffer, chunk *Chunk, sectionY int) {
	bitsPerBlock := byte(4)
	data.WriteByte(bitsPerBlock)

	// 1. 扫描当前 Section，建立本地调色板
	// 1.12.2 Palette 存储的是 (ID << 4) | Metadata
	palette := make([]int32, 0, 16)
	paletteMap := make(map[uint16]int)

	// 默认第一个是空气
	palette = append(palette, 0)
	paletteMap[0] = 0

	baseY := sectionY * 16
	sectionBlocks := make([]uint16, 4096)

	for y := 0; y < 16; y++ {
		wy := baseY + y
		for z := 0; z < 16; z++ {
			for x := 0; x < 16; x++ {
				var state uint16
				if wy >= 0 && wy < 256 {
					state = chunk.Blocks[x][wy][z]
				}

				idx := (y * 256) + (z * 16) + x
				sectionBlocks[idx] = state

				if _, exists := paletteMap[state]; !exists {
					if len(palette) < 16 {
						paletteMap[state] = len(palette)
						palette = append(palette, int32(state))
					}
				}
			}
		}
	}

	// 2. 写入调色板
	Network.WriteVarint(data, int32(len(palette)))
	for _, state := range palette {
		Network.WriteVarint(data, state)
	}

	// 3. 写入数据数组 (Data Array)
	// 1.12.2 非紧凑打包：16 个方块 (4位) 占一个 Long (64位)
	// 4096 个方块共 256 个 Long
	Network.WriteVarint(data, 256)

	for i := 0; i < 256; i++ {
		var val uint64
		for j := 0; j < 16; j++ {
			bidx := i*16 + j
			state := sectionBlocks[bidx]

			paletteIdx := 0
			if pIdx, ok := paletteMap[state]; ok {
				paletteIdx = pIdx
			}

			// 4位打包
			val |= uint64(paletteIdx&0x0F) << uint(j*4)
		}
		// Big-Endian 写入 8 字节
		binary.Write(data, binary.BigEndian, val)
	}

	// 4. 光照数据
	blockLight := make([]byte, 2048)
	skyLight := make([]byte, 2048)
	for i := range skyLight {
		skyLight[i] = 0xFF
	}
	data.Write(blockLight)
	data.Write(skyLight)
}

func wrapPacket(buf *bytes.Buffer) []byte {
	d := buf.Bytes()
	res := &bytes.Buffer{}
	Network.WriteVarint(res, int32(len(d)))
	res.Write(d)

	fmt.Printf("[MapChunk] Packet size: %d bytes (length prefix: %d, data: %d)\n",
		len(res.Bytes()), len(d), len(d))

	return res.Bytes()
}

// BuildChunkUnload 构建区块卸载包 (从 Packet/Play 移过来)
func BuildChunkUnload(chunkX, chunkZ int32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x1D)
	Protocol.WriteInt(buf, chunkX)
	Protocol.WriteInt(buf, chunkZ)
	return wrapPacket(buf)
}
