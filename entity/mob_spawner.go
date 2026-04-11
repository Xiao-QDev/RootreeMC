// Package entity 生物刷怪系统
package entity

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	NoCloseSpawnRadius = 24.0
)

// MobSpawner 生物刷怪器
type MobSpawner struct {
	SpawnInterval  time.Duration // 刷怪间隔
	MaxMobs        int           // 最大生物数量
	SpawnRadius    float64       // 刷怪半径
	DespawnRadius  float64       // 消失半径
	lastSpawn      time.Time
	spawnPositions []SpawnPosition
}

// SpawnPosition 刷怪位置
type SpawnPosition struct {
	X, Y, Z     float64
	MobType     MobType
	Weight      int // 权重（越高越可能生成）
}

// GlobalMobSpawner 全局刷怪器
var GlobalMobSpawner *MobSpawner

func init() {
	GlobalMobSpawner = NewMobSpawner()
}

// NewMobSpawner 创建新的刷怪器
func NewMobSpawner() *MobSpawner {
	spawner := &MobSpawner{
		SpawnInterval: 10 * time.Second, // 每10秒尝试一次
		MaxMobs:       50,               // 最多50个生物
		SpawnRadius:   24,               // 默认生成在玩家24格半径附近
		DespawnRadius: 128,              // 超过128格消失
		lastSpawn:     time.Now(),
	}

	// 初始化刷怪位置（不同生物群系）
	spawner.spawnPositions = []SpawnPosition{
		{MobType: MobTypeZombie, Weight: 100},
		{MobType: MobTypeSkeleton, Weight: 80},
		{MobType: MobTypeCreeper, Weight: 60},
		{MobType: MobTypeSpider, Weight: 70},
		{MobType: MobTypeCow, Weight: 40},
		{MobType: MobTypePig, Weight: 40},
		{MobType: MobTypeChicken, Weight: 35},
		{MobType: MobTypeSheep, Weight: 35},
	}

	return spawner
}

// Player 玩家接口（避免循环导入）
type Player interface {
	GetPosition() (float64, float64, float64)
	GetName() string
	SendPacket(data []byte) error
}

// WorldProvider 世界接口（避免循环导入）
type WorldProvider interface {
	GetBlock(x, y, z int32) uint16
	IsBlockSolid(x, y, z float64) bool
}

// globalWorldProvider 全局世界提供者
var globalWorldProvider WorldProvider

// RegisterWorldProvider 注册世界提供者
func RegisterWorldProvider(provider WorldProvider) {
	globalWorldProvider = provider
}

// Update 每tick更新刷怪器
func (ms *MobSpawner) Update(players []Player) {
	// 检查是否需要刷怪
	if time.Since(ms.lastSpawn) < ms.SpawnInterval {
		return
	}

	// 检查当前生物数量
	currentMobs := len(GlobalEntityManager.GetAllMobs())
	if currentMobs >= ms.MaxMobs {
		return
	}

	// 在玩家附近刷怪
	if len(players) == 0 {
		return
	}

	// 随机选择一个玩家
	player := players[rand.Intn(len(players))]
	
	// 尝试在该玩家附近生成生物
	if ms.TrySpawnNearPlayer(player, players) {
		ms.lastSpawn = time.Now()
	}
}

// TrySpawnNearPlayer 尝试在玩家附近生成生物
func (ms *MobSpawner) TrySpawnNearPlayer(p Player, allPlayers []Player) bool {
	// 随机选择生物类型（按权重）
	mobType := ms.selectMobType()

	// 在玩家周围随机位置生成
	angle := rand.Float64() * 2 * math.Pi
	minDistance := NoCloseSpawnRadius
	maxDistance := ms.SpawnRadius
	if maxDistance < minDistance {
		maxDistance = minDistance
	}
	distance := minDistance
	if maxDistance > minDistance {
		distance += rand.Float64() * (maxDistance - minDistance)
	}

	// 使用接口方法获取玩家位置
	playerX, playerY, playerZ := p.GetPosition()
	x := playerX + math.Cos(angle)*distance
	z := playerZ + math.Sin(angle)*distance
	
	// 查找地面：在玩家高度上下 8 格范围内查找第一个非空气方块
	bx, bz := int32(math.Floor(x)), int32(math.Floor(z))
	y := playerY // 默认
	foundGround := false
	if globalWorldProvider != nil {
		for sy := int32(playerY) + 5; sy >= int32(playerY)-8; sy-- {
			// 必须是可站立的地面方块，避免在液体/草花等非实体方块上刷新
			if !globalWorldProvider.IsBlockSolid(float64(bx)+0.5, float64(sy), float64(bz)+0.5) {
				continue
			}
			// 检查上方是否有 2 格空间
			up1 := globalWorldProvider.GetBlock(bx, sy+1, bz)
			up2 := globalWorldProvider.GetBlock(bx, sy+2, bz)
			if up1 == 0 && up2 == 0 {
				y = float64(sy + 1)
				foundGround = true
				break
			}
		}
	}

	// 用户要求必须在地面刷新
	if !foundGround {
		return false
	}

	// 用户要求：任何生物都必须在亮度 0 生成
	if !isLightZero(bx, int32(y), bz) {
		return false
	}

	// 高度检查
	if y < 1 || y > 255 {
		return false
	}

	// 禁止贴脸刷新：离任意玩家太近都不允许
	if isPlayerTooClose(x, y, z, allPlayers, NoCloseSpawnRadius) {
		return false
	}

	// 检查生成位置是否撞墙 (防止卡进墙里)
	if globalWorldProvider != nil && (globalWorldProvider.IsBlockSolid(x, y, z) || globalWorldProvider.IsBlockSolid(x, y+1.0, z)) {
		return false
	}

	// 检查周围是否有墙 (更严格的生成条件)
	if globalWorldProvider != nil {
		checkPoints := [][]float64{{0.3, 0.3}, {0.3, -0.3}, {-0.3, 0.3}, {-0.3, -0.3}}
		for _, cp := range checkPoints {
			if globalWorldProvider.IsBlockSolid(x+cp[0], y+0.5, z+cp[1]) || globalWorldProvider.IsBlockSolid(x+cp[0], y+1.5, z+cp[1]) {
				return false
			}
		}
	}

	// 创建生物
	eid := GlobalEntityManager.CreateMob(mobType, x, y, z)
	if eid > 0 {
		fmt.Printf("[MobSpawner] 在玩家 %s 附近生成 %s (EID=%d) at (%.1f, %.1f, %.1f)\n",
			p.GetName(), getMobName(mobType), eid, x, y, z)
		
		// 立即广播生成数据包给所有玩家
		mob := GlobalEntityManager.GetMob(eid)
		if mob != nil {
			pkt := BuildSpawnMobPacket(mob)
			for _, receiver := range allPlayers {
				receiver.SendPacket(pkt)
			}
		}
		return true
	}

	return false
}

// isPlayerTooClose 检查刷新点是否离任意玩家过近
func isPlayerTooClose(x, y, z float64, players []Player, minDistance float64) bool {
	minDistanceSq := minDistance * minDistance
	for _, p := range players {
		px, py, pz := p.GetPosition()
		dx := px - x
		dy := py - y
		dz := pz - z
		if dx*dx+dy*dy+dz*dz < minDistanceSq {
			return true
		}
	}
	return false
}

// selectMobType 按权重随机选择生物类型
func (ms *MobSpawner) selectMobType() MobType {
	totalWeight := 0
	for _, pos := range ms.spawnPositions {
		totalWeight += pos.Weight
	}

	randWeight := rand.Intn(totalWeight)
	currentWeight := 0

	for _, pos := range ms.spawnPositions {
		currentWeight += pos.Weight
		if randWeight < currentWeight {
			return pos.MobType
		}
	}

	return MobTypeZombie // 默认
}

// DespawnFarMobs 移除距离玩家太远的生物
func (ms *MobSpawner) DespawnFarMobs(players []Player) {
	if len(players) == 0 {
		return
	}

	mobs := GlobalEntityManager.GetAllMobs()
	for _, mob := range mobs {
		// 检查与最近玩家的距离
		minDistance := ms.DespawnRadius + 1
		for _, p := range players {
			x, y, z := p.GetPosition()
			distance := calculateDistance(mob.X, mob.Y, mob.Z, x, y, z)
			if distance < minDistance {
				minDistance = distance
			}
		}

		// 如果距离所有玩家都超过消失半径，则移除
		if minDistance > ms.DespawnRadius {
			GlobalEntityManager.RemoveMob(mob.EID)
			
			// 广播销毁包
			pkt := BuildDestroyEntities([]int32{mob.EID})
			for _, p := range players {
				p.SendPacket(pkt)
			}
		}
	}
}

// isHostileMob 判断是否为敌对生物
func isHostileMob(mobType MobType) bool {
	switch mobType {
	case MobTypeZombie, MobTypeSkeleton, MobTypeCreeper, MobTypeSpider:
		return true
	default:
		return false
	}
}

// getMobName 获取生物名称
func getMobName(mobType MobType) string {
	switch mobType {
	case MobTypeZombie:
		return "Zombie"
	case MobTypeSkeleton:
		return "Skeleton"
	case MobTypeCreeper:
		return "Creeper"
	case MobTypeSpider:
		return "Spider"
	case MobTypeCow:
		return "Cow"
	case MobTypePig:
		return "Pig"
	case MobTypeChicken:
		return "Chicken"
	case MobTypeSheep:
		return "Sheep"
	default:
		return "Unknown"
	}
}

// SpawnMob 手动生成生物（命令用）
func SpawnMob(mobType MobType, x, y, z float64) int32 {
	eid := GlobalEntityManager.CreateMob(mobType, x, y, z)
	if eid > 0 {
		fmt.Printf("[SpawnMob] 手动生成 %s (EID=%d) at (%.1f, %.1f, %.1f)\n",
			getMobName(mobType), eid, x, y, z)
	}
	return eid
}

// isLightZero 检查是否为亮度 0
func isLightZero(x, y, z int32) bool {
	// 由于没有光照引擎，通过以下逻辑模拟：
	// 1. 如果上方有固体方块，则亮度为 0
	// 2. 如果是黑夜，天空亮度较低，但完全为 0 仍需有顶棚覆盖 (模拟更严格的 0 亮度)
	
	// 检查该位置往上是否有任何非空气方块 (Roof Check)
	hasRoof := false
	if globalWorldProvider != nil {
		for sy := y + 1; sy < 256; sy++ {
			block := globalWorldProvider.GetBlock(x, sy, z)
			if block != 0 {
				hasRoof = true
				break
			}
		}
	}
	
	// 用户要求“在亮度 0 的地方生成”
	// 物理意义：如果没有顶棚，天空光总会有的。所以必须有顶棚。
	return hasRoof
}

// UpdateMobAI 更新所有生物AI
func UpdateMobAI() {
	mobs := GlobalEntityManager.GetAllMobs()
	ai := &DefaultMobAI{}

	for _, mob := range mobs {
		ai.Update(mob)
	}
}
