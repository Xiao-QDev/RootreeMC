// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
)

// JoinGamePacket 加入游戏包 - 服务器发送此包让玩家进入游戏世界
type JoinGamePacket struct {
	EntityID         int32  // 玩家的实体 ID
	Gamemode         byte   // 游戏模式: 0=生存, 1=创造, 2=冒险, 3=旁观者
	Dimension        int32  // 维度: -1=下界, 0=主世界, 1=末地
	Difficulty       byte   // 难度: 0=和平, 1=简单, 2=普通, 3=困难
	MaxPlayers       byte   // 最大玩家数（仅用于显示，不影响实际限制）
	LevelType        string // 世界类型: "default", "flat", "largeBiomes" 等
	ReducedDebugInfo bool   // 是否减少调试信息（F3 显示的信息）
}

// BuildJoinGame 构建加入游戏包
// 参数: pkt - 加入游戏包结构体
// 返回: 完整的加入游戏包字节数据（包含长度前缀）
// Minecraft 1.12.2 (协议 340) Join Game 包 ID: 0x23
func BuildJoinGame(pkt *JoinGamePacket) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x23) // Packet ID (Play 状态, 1.12.2 = 0x23)
	Protocol.WriteInt(buf, pkt.EntityID)
	buf.WriteByte(pkt.Gamemode)
	Protocol.WriteInt(buf, pkt.Dimension)
	buf.WriteByte(pkt.Difficulty)
	buf.WriteByte(pkt.MaxPlayers)
	Protocol.WriteString(buf, pkt.LevelType)
	Protocol.WriteBoolean(buf, pkt.ReducedDebugInfo)

	return Protocol.AddLengthPrefix(buf)
}

// BuildDefaultJoinGame 构建默认的加入游戏包（常用配置）
// 参数: entityID - 玩家实体 ID
// 返回: 使用默认配置的加入游戏包字节数据
func BuildDefaultJoinGame(entityID int32) []byte {
	return BuildJoinGame(&JoinGamePacket{
		EntityID:         entityID,
		Gamemode:         1,         // 创造模式（更容易测试）
		Dimension:        0,         // 主世界 (Overworld)
		Difficulty:       2,         // 普通难度
		MaxPlayers:       20,        // 最大 20 人
		LevelType:        "default", // 默认世界类型
		ReducedDebugInfo: false,     // 显示完整调试信息
	})
}
