// Package Play 游戏时间同步包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// BuildTimeUpdate 发送时间更新包
// Minecraft 1.12.2 (协议 340) Time Update 包 ID: 0x38
// 用于同步游戏时间（白天/黑夜）
// 返回: [Length][PacketID][Data]
func BuildTimeUpdate(worldAge int64, timeOfDay int64) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x47)     // Packet ID: 0x47
	Protocol.WriteLong(buf, worldAge)  // 世界年龄（总时间，不受日夜循环影响）
	Protocol.WriteLong(buf, timeOfDay) // 当天时间（0-24000，白天为0，夜晚为13000）
	return Protocol.AddLengthPrefix(buf)
}

// BuildDayTime 设置为白天
func BuildDayTime() []byte {
	return BuildTimeUpdate(0, 1000) // 早上
}

// BuildNightTime 设置为夜晚
func BuildNightTime() []byte {
	return BuildTimeUpdate(0, 13000) // 夜晚开始
}
