// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// PlayerAbilitiesPacket 玩家能力包 - 设置玩家的能力标志
type PlayerAbilitiesPacket struct {
	Flags       byte    // 能力标志位: bit0=无敌, bit1=飞行中, bit2=允许飞行, bit3=创造模式
	FlyingSpeed float32 // 飞行速度 (默认 0.05)
	WalkSpeed   float32 // 行走速度 (默认 0.1)
}

// BuildPlayerAbilities 构建玩家能力包
// 参数: flags - 能力标志, flyingSpeed - 飞行速度, walkSpeed - 行走速度
// 返回: 完整的玩家能力包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) Player Abilities 包 ID: 0x2C
func BuildPlayerAbilities(flags byte, flyingSpeed, walkSpeed float32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2C) // Packet ID: 0x2C
	buf.WriteByte(flags)
	Protocol.WriteFloat(buf, flyingSpeed)
	Protocol.WriteFloat(buf, walkSpeed)
	return Protocol.AddLengthPrefix(buf)
}

// BuildDefaultAbilities 构建默认的玩家能力（生存模式）
// 返回: 使用默认配置的生存模式能力包
func BuildDefaultAbilities() []byte {
	return BuildPlayerAbilities(0x00, 0.05, 0.1)
}

// BuildCreativeAbilities 构建创造模式能力
// 返回: 创造模式的玩家能力包
func BuildCreativeAbilities() []byte {
	return BuildPlayerAbilities(0x0E, 0.05, 0.1) // 0x0E = 允许飞行 + 正在飞行 + 创造模式
}
