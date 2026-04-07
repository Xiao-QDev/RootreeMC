// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// UpdateHealthPacket 更新生命值包 - 设置玩家的生命值、饱食度和饱和度
type UpdateHealthPacket struct {
	Health     float32 // 生命值 (0-20, 0=死亡)
	Food       int32   // 饱食度 (0-20)
	Saturation float32 // 饱和度 (隐藏值，影响饱食度消耗)
}

// BuildUpdateHealth 构建更新生命值包
// 参数: health - 生命值, food - 饱食度, saturation - 饱和度
// 返回: 完整的更新生命值包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) Update Health 包 ID: 0x41
func BuildUpdateHealth(health float32, food int32, saturation float32) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x41) // Packet ID: 0x41
	Protocol.WriteFloat(buf, health)
	Network.WriteVarint(buf, food)
	Protocol.WriteFloat(buf, saturation)
	return Protocol.AddLengthPrefix(buf)
}

// BuildFullHealth 构建满血状态的更新包
// 返回: 满生命值和满饱食度的数据包
func BuildFullHealth() []byte {
	return BuildUpdateHealth(
		20.0, // 满生命值
		20,   // 满饱食度
		5.0,  // 高饱和度
	)
}
