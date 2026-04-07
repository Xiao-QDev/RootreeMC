// Package command 命令系统
package command

import (
	"RootreeMC/Network"
	"RootreeMC/Protocol"
	"RootreeMC/entity"
	"RootreeMC/player"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Handler 命令处理函数
type Handler func(p *player.OnlinePlayer, args []string) bool

// handlers 命令处理函数映射
var handlers = make(map[string]Handler)

// Register 注册命令处理函数
func Register(name string, handler Handler) {
	handlers[name] = handler
}

// Handle 处理玩家命令
func Handle(p *player.OnlinePlayer, command string) bool {
	if len(command) == 0 || command[0] != '/' {
		return false
	}

	parts := strings.Fields(command[1:])
	if len(parts) == 0 {
		return false
	}

	cmdName := parts[0]
	args := parts[1:]

	handler, exists := handlers[cmdName]
	if !exists {
		return false
	}

	return handler(p, args)
}

// BroadcastMessage 广播消息给所有在线玩家
func BroadcastMessage(message string, exclude *Network.Network) {
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: fmt.Sprintf("§e%s§r", message),
	}
	jsonBytes, _ := json.Marshal(msgObj)
	pkt := Protocol.BuildSystemMessage(string(jsonBytes))

	// 获取所有在线玩家并发送
	for _, online := range player.GlobalPlayerManager.GetAllOnlinePlayers() {
		if online.Client != exclude {
			online.Client.Send(pkt)
		}
	}
}

// SendMessage 发送消息给指定玩家 (自动包装为 JSON)
func SendMessage(p *player.OnlinePlayer, message string) {
	msgObj := struct {
		Text string `json:"text"`
	}{
		Text: message,
	}
	jsonBytes, _ := json.Marshal(msgObj)
	pkt := Protocol.BuildSystemMessage(string(jsonBytes))
	p.Client.Send(pkt)
}

func init() {
	// 注册内置命令
	Register("tp", handleTp)
	Register("gamemode", handleGamemode)
	Register("give", handleGive)
	Register("help", handleHelp)
	Register("tps", handleTps)
	Register("drop", handleDrop)
	Register("spawn", handleSpawn)
	Register("light", handleLight)
}

// /tps - 显示当前服务器性能指数
func handleTps(p *player.OnlinePlayer, args []string) bool {
	tps := player.GetTPS()
	color := "§a" // 绿色 (正常)
	if tps < 18.0 {
		color = "§e" // 黄色 (警告)
	}
	if tps < 15.0 {
		color = "§c" // 红色 (卡顿)
	}

	SendMessage(p, fmt.Sprintf("§6服务器性能状态:"))
	SendMessage(p, fmt.Sprintf("  §7当前 TPS: %s%.2f §r/ 20.00", color, tps))

	// 计算负载百分比
	load := (tps / 20.0) * 100
	SendMessage(p, fmt.Sprintf("  §7负载: %s%.1f%%", color, load))

	return true
}

// /tp <x> <y> <z> - 传送
func handleTp(p *player.OnlinePlayer, args []string) bool {
	if len(args) != 3 {
		SendMessage(p, "§c用法: /tp <x> <y> <z>")
		return true
	}

	var x, y, z float64
	if _, err := fmt.Sscanf(args[0], "%f", &x); err != nil {
		SendMessage(p, "§c无效的 X 坐标")
		return true
	}
	if _, err := fmt.Sscanf(args[1], "%f", &y); err != nil {
		SendMessage(p, "§c无效的 Y 坐标")
		return true
	}
	if _, err := fmt.Sscanf(args[2], "%f", &z); err != nil {
		SendMessage(p, "§c无效的 Z 坐标")
		return true
	}

	// 更新玩家位置
	p.PlayerEntity.X = x
	p.PlayerEntity.Y = y
	p.PlayerEntity.Z = z

	// 发送传送包
	teleportPkt := Protocol.BuildAbsoluteTeleport(x, y, z, p.PlayerEntity.Yaw, p.PlayerEntity.Pitch, 0)
	p.Client.Send(teleportPkt)

	SendMessage(p, fmt.Sprintf("§e传送到 (%.1f, %.1f, %.1f)", x, y, z))
	return true
}

// /gamemode <0|1|2|3> - 切换游戏模式
func handleGamemode(p *player.OnlinePlayer, args []string) bool {
	if len(args) != 1 {
		SendMessage(p, "§c用法: /gamemode <0|1|2|3>")
		return true
	}

	var gamemode int32
	if _, err := fmt.Sscanf(args[0], "%d", &gamemode); err != nil || gamemode < 0 || gamemode > 3 {
		SendMessage(p, "§c无效的游戏模式 (必须是 0-3)")
		return true
	}

	p.PlayerEntity.Gamemode = gamemode

	// 发送游戏状态变更
	statePkt := Protocol.BuildChangeGameState(3, float32(gamemode))
	p.Client.Send(statePkt)

	modes := []string{"生存", "创造", "冒险", "旁观"}
	SendMessage(p, fmt.Sprintf("§a游戏模式切换到 %s", modes[gamemode]))
	return true
}

// /give <item> [amount] - 给予物品
func handleGive(p *player.OnlinePlayer, args []string) bool {
	if len(args) < 1 {
		SendMessage(p, "§c用法: /give <item> [amount]")
		return true
	}

	itemName := args[0]
	amount := 1
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &amount)
	}

	if amount > 64 {
		amount = 64
	}

	itemID := parseItemID(itemName)
	if itemID == 0 {
		SendMessage(p, fmt.Sprintf("§c未知物品: %s", itemName))
		return true
	}

	item := player.NewItemStack(int32(itemID), byte(amount))
	if p.Inventory.AddItem(item) {
		SendMessage(p, fmt.Sprintf("§e获得 %d 个 %s", amount, itemName))
		return true
	}

	SendMessage(p, "§c物品栏已满")
	return true
}

// /help - 显示帮助
func handleHelp(p *player.OnlinePlayer, args []string) bool {
	help := []string{
		"§6可用命令:",
		"§e/tp <x> <y> <z> §7- 传送到指定位置",
		"§e/gamemode <0|1|2|3> §7- 切换游戏模式",
		"§e/give <item> [amount] §7- 给予物品",
		"§e/spawn <mob> §7- 生成生物 (zombie, skeleton, creeper, spider, cow, pig, chicken, sheep)",
		"§e/light §7- 更新玩家所在区块的光照",
		"§e/drop §7- 丢弃一个钻石(测试用)",
		"§e/tps §7- 显示服务器 TPS 性能指数",
		"§e/help §7- 显示此帮助信息",
	}

	for _, line := range help {
		SendMessage(p, line)
	}
	return true
}

// /drop - 丢弃一个钻石（测试用）
func handleDrop(p *player.OnlinePlayer, args []string) bool {
	// 创建钻石物品
	_ = player.NewItemStack(264, 1) // 钻石

	// 在玩家位置生成掉落物
	posX := p.PlayerEntity.X
	posY := p.PlayerEntity.Y
	posZ := p.PlayerEntity.Z

	// 随机偏移位置，避免直接生成在玩家体内
	offsetX := (float64(p.PlayerEntity.EID%10) - 5.0) / 10.0
	offsetZ := (float64(p.PlayerEntity.EID%7) - 3.5) / 10.0

	eid := entity.GlobalEntityManager.CreateItemEntity(
		264, // 钻石
		1,   // 数量
		nil, // 无NBT
		posX+offsetX, posY, posZ+offsetZ,
		0, 0.1, 0, // 微小的向上速度
	)

	if eid > 0 {
		SendMessage(p, fmt.Sprintf("§e丢弃了钻石! 掉落物EID=%d", eid))
		fmt.Printf("[Drop] 玩家 %s 丢弃物品: 钻石, EID=%d\n", p.Username, eid)

		// 广播给所有玩家
		itemEntity := entity.GlobalEntityManager.GetItemEntity(eid)
		if itemEntity != nil {
			// 1. 发送 Spawn Item Entity 包
			spawnItemPkt := entity.BuildSpawnItemEntity(itemEntity)
			allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
			for _, onlineP := range allPlayers {
				onlineP.Client.Send(spawnItemPkt)
			}

			// 2. 发送 Entity Metadata 包（设置物品堆叠）
			metaBuf := &bytes.Buffer{}
			Network.WriteVarint(metaBuf, 0x39) // Packet ID: Entity Metadata
			Network.WriteVarint(metaBuf, itemEntity.EID)

			// 元数据: Index 6, Type 5 (Slot)
			metaBuf.WriteByte(6) // Index
			metaBuf.WriteByte(5) // Type

			// Slot数据
			Network.WriteVarint(metaBuf, 264) // 钻石
			metaBuf.WriteByte(1)              // Count
			metaBuf.WriteByte(0)              // NBT: TAG_End
			metaBuf.WriteByte(0xFF)           // 结束

			metaPkt := Protocol.AddLengthPrefix(metaBuf)
			for _, onlineP := range allPlayers {
				onlineP.Client.Send(metaPkt)
			}
		}
	} else {
		SendMessage(p, "§c丢弃物品失败")
	}

	return true
}

// parseItemID 解析物品名称到ID
func parseItemID(name string) int32 {
	switch strings.ToLower(name) {
	case "stone":
		return 1
	case "dirt":
		return 3
	case "wood", "planks":
		return 5
	case "glass":
		return 20
	case "torch":
		return 50
	case "tnt":
		return 46
	case "chest":
		return 54
	case "diamond":
		return 264
	case "iron_ingot":
		return 265
	case "gold_ingot":
		return 266
	}
	return 0
}

// /spawn <mob> - 生成生物
func handleSpawn(p *player.OnlinePlayer, args []string) bool {
	if len(args) < 1 {
		SendMessage(p, "§c用法: /spawn <mobtype>")
		SendMessage(p, "§7生物类型: zombie, skeleton, creeper, spider, cow, pig, chicken, sheep")
		return true
	}

	mobType := parseMobType(args[0])
	if mobType == -1 {
		SendMessage(p, "§c未知的生物类型: "+args[0])
		return true
	}

	// 在玩家前方3格生成
	yawRad := float64(p.PlayerEntity.Yaw) * math.Pi / 180.0
	offsetX := -math.Sin(yawRad) * 3.0
	offsetZ := math.Cos(yawRad) * 3.0

	x := p.PlayerEntity.X + offsetX
	y := p.PlayerEntity.Y
	z := p.PlayerEntity.Z + offsetZ

	eid := entity.SpawnMob(mobType, x, y, z)
	if eid > 0 {
		SendMessage(p, fmt.Sprintf("§e生成 %s (EID=%d)", getMobTypeName(mobType), eid))
	} else {
		SendMessage(p, "§c生成失败")
	}

	return true
}

// parseMobType 解析生物类型
func parseMobType(name string) entity.MobType {
	switch strings.ToLower(name) {
	case "zombie", "僵尸":
		return entity.MobTypeZombie
	case "skeleton", "骷髅":
		return entity.MobTypeSkeleton
	case "creeper", "爬行者", "苦力怕":
		return entity.MobTypeCreeper
	case "spider", "蜘蛛":
		return entity.MobTypeSpider
	case "cow", "牛":
		return entity.MobTypeCow
	case "pig", "猪":
		return entity.MobTypePig
	case "chicken", "鸡":
		return entity.MobTypeChicken
	case "sheep", "羊":
		return entity.MobTypeSheep
	}
	return -1
}

// getMobTypeName 获取生物类型名称
func getMobTypeName(mobType entity.MobType) string {
	switch mobType {
	case entity.MobTypeZombie:
		return "Zombie"
	case entity.MobTypeSkeleton:
		return "Skeleton"
	case entity.MobTypeCreeper:
		return "Creeper"
	case entity.MobTypeSpider:
		return "Spider"
	case entity.MobTypeCow:
		return "Cow"
	case entity.MobTypePig:
		return "Pig"
	case entity.MobTypeChicken:
		return "Chicken"
	case entity.MobTypeSheep:
		return "Sheep"
	}
	return "Unknown"
}

// LightUpdateCallback 光照更新回调函数类型
type LightUpdateCallback func(chunkX, chunkZ int32)

var lightUpdateCallback LightUpdateCallback

// RegisterLightUpdateCallback 注册光照更新回调
func RegisterLightUpdateCallback(callback LightUpdateCallback) {
	lightUpdateCallback = callback
}

// /light - 更新玩家所在区块的光照
func handleLight(p *player.OnlinePlayer, args []string) bool {
	// 获取玩家位置
	chunkX := int32(p.PlayerEntity.X) / 16
	chunkZ := int32(p.PlayerEntity.Z) / 16

	// 计算该区块的自然光照（通过回调避免循环导入）
	if lightUpdateCallback != nil {
		lightUpdateCallback(chunkX, chunkZ)
	}

	SendMessage(p, fmt.Sprintf("§e已更新区块 (%d, %d) 的光照", chunkX, chunkZ))
	return true
}
