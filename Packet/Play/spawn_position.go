// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/binary"
)

// SpawnPositionPacket 出生点位置包 - 设置玩家的重生点
type SpawnPositionPacket struct {
	Location int64 // 编码后的位置 (x:26位, z:26位, y:12位)
}

// BuildSpawnPosition 构建出生点位置包
// 参数: x, y, z - 出生点坐标
// 返回: 完整的出生点位置包字节数据 (包含长度前缀)
// Minecraft 1.12.2 (协议 340) Spawn Position 包 ID: 0x46
func BuildSpawnPosition(x, y, z int32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x46) // Packet ID: 0x46 (Spawn Position)
	
	// 1.12.2 协议: Position 是 8 字节 Long (编码后的位置)
	location := Protocol.EncodePosition(x, y, z)
	Protocol.WriteLong(buf, int64(binary.BigEndian.Uint64(location[:])))
	
	return Protocol.AddLengthPrefix(buf)
}
