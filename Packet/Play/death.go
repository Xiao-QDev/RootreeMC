// Package Play Minecraft 游戏阶段协议包
package Play

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"bytes"
	"encoding/json"
)

// BuildCombatEventDeath 构建玩家死亡事件包（Combat Event: Entity Dead）
// Minecraft 1.12.2 (协议 340) Clientbound Packet ID: 0x2D
func BuildCombatEventDeath(playerEID int32, killerEID int32, deathMessage string) []byte {
	buf := &bytes.Buffer{}
	Network.WriteVarint(buf, 0x2D) // Packet ID
	Network.WriteVarint(buf, 2)    // Event: Entity Dead

	// Player ID (VarInt) + Killer ID (Int)
	Network.WriteVarint(buf, playerEID)
	Protocol.WriteInt(buf, killerEID)

	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: deathMessage,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	Protocol.WriteString(buf, string(jsonBytes))

	return Protocol.AddLengthPrefix(buf)
}
