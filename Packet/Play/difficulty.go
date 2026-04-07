// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// DifficultyPacket 难度包 - 设置游戏难度
type DifficultyPacket struct {
	Difficulty       byte // 难度: 0=和平, 1=简单, 2=普通, 3=困难
	DifficultyLocked bool // 难度是否锁定
}

// BuildDifficulty 构建难度包
// 参数: difficulty - 难度等级
// 返回: 完整的难度包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) Server Difficulty 包 ID: 0x0D
func BuildDifficulty(difficulty byte) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x0D) // Packet ID: 0x0D
	buf.WriteByte(difficulty)
	return Protocol.AddLengthPrefix(buf)
}
