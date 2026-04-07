package Tick

import (
	"RootreeMC/Packet/Play"
	"RootreeMC/Protocol"
	"RootreeMC/entity"
	"RootreeMC/player"
	"RootreeMC/world"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// ====== 物理常量 (与Vanilla一致) ======
const (
	TPS              = 20                  // 20 ticks/秒
	TickDuration     = time.Second / TPS   // 50ms/tick
	Gravity          = -0.08               // 方块/tick²
	Drag             = 0.98                // 空气阻力
	TerminalVelocity = -3.92               // 终端速度 方块/tick
	PlayerHeight     = 1.62                // 玩家眼睛高度(碰撞箱顶部1.8，但眼睛1.62)
	PlayerWidth      = 0.6                 // 碰撞箱宽度
	MaxFallDistNoDmg = 3.0                 // 无伤掉落最大距离(方块)
	TimeSyncInterval = 20                  // 每20tick同步一次时间(1秒)
	SaveInterval     = int64(TPS * 60 * 5) // 每5分钟自动保存
)

// TickMode tick运行模式
type TickMode int

const (
	TickModeSingleThread TickMode = iota // 单线程模式
	TickModeMultiThread                  // 多线程模式
)

// ====== 全局游戏状态 ======

type GameState struct {
	mu         sync.RWMutex
	WorldAge   int64    // 世界总tick数
	TimeOfDay  int64    // 昼夜循环 0~24000
	Running    bool
	tickCount  int64
	Mode       TickMode // tick模式
	CurrentTPS float64  // 当前TPS
}

var (
	GlobalGameState *GameState
	lastTickTimes   []time.Time
	tickTimesMu     sync.Mutex
)

func init() {
	GlobalGameState = &GameState{
		WorldAge:   0,
		TimeOfDay:  1000, // 默认早晨 (~6:00)
		Running:    false,
		Mode:       TickModeSingleThread, // 默认单线程
		CurrentTPS: 20.0,
	}
	lastTickTimes = make([]time.Time, 0, 20)
}

// Start 启动游戏主循环（阻塞，应在独立goroutine中调用）
func Start() {
	GlobalGameState.mu.Lock()
	if GlobalGameState.Running {
		GlobalGameState.mu.Unlock()
		return
	}
	GlobalGameState.Running = true
	GlobalGameState.mu.Unlock()

	slog.Info("[Tick] 游戏主循环已启动", slog.Int("TPS", TPS))

	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()

	for {
		<-ticker.C

		GlobalGameState.mu.RLock()
		running := GlobalGameState.Running
		GlobalGameState.mu.RUnlock()

		if !running {
			return
		}

		runSingleTick()
	}
}

// Stop 优雅停止游戏主循环
func Stop() {
	GlobalGameState.mu.Lock()
	defer GlobalGameState.mu.Unlock()
	GlobalGameState.Running = false
	slog.Info("[Tick] 游戏主循环已停止")
}

// GetCurrentTPS 获取当前 TPS
func GetCurrentTPS() float64 {
	GlobalGameState.mu.RLock()
	defer GlobalGameState.mu.RUnlock()
	return GlobalGameState.CurrentTPS
}

// GetTimeOfDay 获取当前时间（线程安全）
func GetTimeOfDay() int64 {
	GlobalGameState.mu.RLock()
	defer GlobalGameState.mu.RUnlock()
	return GlobalGameState.TimeOfDay
}

// SetTimeOfDay 设置当前时间
func SetTimeOfDay(time int64) {
	GlobalGameState.mu.Lock()
	defer GlobalGameState.mu.Unlock()
	GlobalGameState.TimeOfDay = time % 24000
	if GlobalGameState.TimeOfDay < 0 {
		GlobalGameState.TimeOfDay += 24000
	}
}

// SetTickMode 设置tick模式
func SetTickMode(mode TickMode) {
	GlobalGameState.mu.Lock()
	defer GlobalGameState.mu.Unlock()
	GlobalGameState.Mode = mode
}

// GetTickMode 获取当前tick模式
func GetTickMode() TickMode {
	GlobalGameState.mu.RLock()
	defer GlobalGameState.mu.RUnlock()
	return GlobalGameState.Mode
}

// GetWorldAge 获取世界总tick数
func GetWorldAge() int64 {
	GlobalGameState.mu.RLock()
	defer GlobalGameState.mu.RUnlock()
	return GlobalGameState.WorldAge
}

// IsRunning 返回tick循环是否在运行
func IsRunning() bool {
	GlobalGameState.mu.RLock()
	defer GlobalGameState.mu.RUnlock()
	return GlobalGameState.Running
}

// ====== 核心Tick逻辑 ======

func runSingleTick() {
	// --- TPS 计算 ---
	tickTimesMu.Lock()
	nowTime := time.Now()
	lastTickTimes = append(lastTickTimes, nowTime)
	if len(lastTickTimes) > 20 {
		lastTickTimes = lastTickTimes[1:]
	}
	if len(lastTickTimes) >= 2 {
		duration := lastTickTimes[len(lastTickTimes)-1].Sub(lastTickTimes[0])
		if duration > 0 {
			tps := float64(len(lastTickTimes)-1) / duration.Seconds()
			if tps > 20.0 {
				tps = 20.0
			}
			GlobalGameState.mu.Lock()
			GlobalGameState.CurrentTPS = tps
			GlobalGameState.mu.Unlock()
			player.SetTPS(tps)
		}
	}
	tickTimesMu.Unlock()

	GlobalGameState.mu.Lock()
	GlobalGameState.tickCount++
	now := GlobalGameState.tickCount

	// --- 时间推进 ---
	GlobalGameState.WorldAge++
	GlobalGameState.TimeOfDay++
	if GlobalGameState.TimeOfDay >= 24000 {
		GlobalGameState.TimeOfDay = 0
	}
	GlobalGameState.mu.Unlock()

	// --- 定时任务 ---

	// 1. 时间同步 (每秒1次，避免网络拥塞)
	if now%int64(TimeSyncInterval) == 0 {
		broadcastTimeUpdateSafe()
	}

	// 2. 玩家物理
	processAllPlayerPhysics()

	// 3. Keep Alive超时检查 (每秒检查一次)
	if now%int64(TPS) == 0 {
		player.CheckKeepAliveTimeout()
	}

	// 4. 自动存档 (每5分钟)
	if now%SaveInterval == 0 {
		player.GlobalPlayerManager.SaveAllPlayers()
		slog.Info("[Tick] 自动保存完成")
	}

	// 5. 状态日志 (每30秒)
	if now%int64(TPS*30) == 0 {
		onlineCount := len(player.GlobalPlayerManager.GetAllOnlinePlayers())
		slog.Info("[Tick] 心跳",
			slog.Int64("tick", now),
			slog.Int64("worldAge", GlobalGameState.WorldAge),
			slog.Int64("timeOfDay", GlobalGameState.TimeOfDay),
			slog.Int("players", onlineCount),
		)
	}

	// 6. 更新生物AI
	entity.UpdateMobAI()

	// 7. 处理生物物理和同步
	processMobPhysics()

	// 7. 更新生物刷怪器
	allPlayers := player.GlobalPlayerManager.GetAllOnlinePlayers()
	playerInterfaces := make([]entity.Player, len(allPlayers))
	for i, p := range allPlayers {
		playerInterfaces[i] = p // OnlinePlayer实现了entity.Player接口
	}
	entity.GlobalMobSpawner.Update(playerInterfaces)
	entity.GlobalMobSpawner.DespawnFarMobs(playerInterfaces)
}

// ====== 时间同步 ======

// broadcastTimeUpdateSafe 安全地向所有在线玩家广播时间更新
func broadcastTimeUpdateSafe() {
	GlobalGameState.mu.RLock()
	worldAge := GlobalGameState.WorldAge
	timeOfDay := GlobalGameState.TimeOfDay
	GlobalGameState.mu.RUnlock()

	// 只在worldAge > 100后发送（约5秒），确保玩家已完全进入
	if worldAge < 100 {
		return
	}

	// BuildTimeUpdate 返回已经带有长度前缀的字节数组 [Length][PacketID][Data]
	pkt := Play.BuildTimeUpdate(worldAge, timeOfDay)

	fmt.Printf("[TimeUpdate] worldAge=%d timeOfDay=%d totalLen=%d\n",
		worldAge, timeOfDay, len(pkt))

	// 只在有玩家在线时发送
	players := player.GlobalPlayerManager.GetAllOnlinePlayers()
	if len(players) == 0 {
		return
	}

	for _, p := range players {
		if err := p.Client.Send(pkt); err != nil {
			fmt.Printf("[Tick/Time] Failed to send to %s: %v\n", p.Username, err)
		}
	}
}

// ====== 玩家物理系统 ======

// PlayerVelocity 三轴速度
type PlayerVelocity struct {
	VX, VY, VZ float64
}

// PlayerPhysics 玩家物理状态（每个在线玩家一份）
type PlayerPhysics struct {
	Velocity     PlayerVelocity
	FallDistance float64 // 当前连续掉落距离(方块)
	OnGround     bool
	LastY        float64 // 上一tick的Y坐标
	Initialized  bool
}

var (
	physMap = make(map[int32]*PlayerPhysics) // EID -> 物理
	physMu  sync.RWMutex
)

// getOrCreatePhysics 获取或创建玩家物理数据
func getOrCreatePhysics(eid int32) *PlayerPhysics {
	physMu.RLock()
	p, ok := physMap[eid]
	physMu.RUnlock()
	if ok {
		return p
	}

	physMu.Lock()
	defer physMu.Unlock()
	// 双重检查
	if p, ok = physMap[eid]; ok {
		return p
	}
	p = &PlayerPhysics{
		Velocity:     PlayerVelocity{0, 0, 0},
		FallDistance: 0,
		OnGround:     true,
		LastY:        0,
		Initialized:  false,
	}
	physMap[eid] = p
	return p
}

// RemovePlayerPhysics 清理离线玩家的物理数据（供PlayerLeave调用）
func RemovePlayerPhysics(eid int32) {
	physMu.Lock()
	delete(physMap, eid)
	physMu.Unlock()
}

// processAllPlayerPhysics 处理所有在线玩家物理（支持单线程/多线程模式）
func processAllPlayerPhysics() {
	players := player.GlobalPlayerManager.GetAllOnlinePlayers()
	
	GlobalGameState.mu.RLock()
	mode := GlobalGameState.Mode
	GlobalGameState.mu.RUnlock()
	
	if mode == TickModeMultiThread {
		// 多线程模式：为每个玩家启动goroutine
		var wg sync.WaitGroup
		for _, p := range players {
			wg.Add(1)
			go func(player *player.OnlinePlayer) {
				defer wg.Done()
				tickPlayerPhysics(player)
			}(p)
		}
		wg.Wait()
	} else {
		// 单线程模式：顺序处理
		for _, p := range players {
			tickPlayerPhysics(p)
		}
	}
	
	// 处理掉落物实体
	processItemEntities()
}

// processMobPhysics 处理生物物理移动和广播
func processMobPhysics() {
	mobs := entity.GlobalEntityManager.GetAllMobs()
	players := player.GlobalPlayerManager.GetAllOnlinePlayers()
	if len(mobs) == 0 || len(players) == 0 {
		return
	}

	for _, mob := range mobs {
		// --- 1. 垂直物理 (重力) ---
		if !mob.OnGround {
			mob.VelocityY += Gravity
			if mob.VelocityY < TerminalVelocity {
				mob.VelocityY = TerminalVelocity
			}
		} else {
			if mob.VelocityY < 0 {
				mob.VelocityY = 0
			}
		}

		// --- 2. 水平物理 (墙壁碰撞) ---
		// 检查 X 轴移动 (带碰撞半径)
		newX := mob.X + mob.VelocityX
		offsetX := 0.3
		if mob.VelocityX < 0 {
			offsetX = -0.3
		}
		if !world.IsBlockSolid(newX+offsetX, mob.Y+0.1, mob.Z) && !world.IsBlockSolid(newX+offsetX, mob.Y+1.1, mob.Z) {
			mob.X = newX
		} else {
			mob.VelocityX = 0 // 撞墙停止
		}

		// 检查 Z 轴移动 (带碰撞半径)
		newZ := mob.Z + mob.VelocityZ
		offsetZ := 0.3
		if mob.VelocityZ < 0 {
			offsetZ = -0.3
		}
		if !world.IsBlockSolid(mob.X, mob.Y+0.1, newZ+offsetZ) && !world.IsBlockSolid(mob.X, mob.Y+1.1, newZ+offsetZ) {
			mob.Z = newZ
		} else {
			mob.VelocityZ = 0 // 撞墙停止
		}

		// 施加垂直位移
		mob.Y += mob.VelocityY

		// 3. 地面碰撞检测与修正
		// 检测脚下是否是固体方块
		onGround := world.IsOnGround(mob.X, mob.Y, mob.Z)
		justLanded := !mob.OnGround && onGround
		mob.OnGround = onGround

		// 如果在地面上，确保 Y 坐标对齐到方块表面
		if onGround {
			// checkOnGround 检测的是 Floor(Y - 0.05) 位置的方块
			// 既然在地面上，Y 坐标应该是该方块的顶面，即 Floor(Y - 0.05) + 1
			mob.Y = math.Floor(mob.Y-0.05) + 1.0
		}

		// 4. 广播位置更新 (仅当移动或状态改变时)
		if mob.VelocityX != 0 || mob.VelocityY != 0 || mob.VelocityZ != 0 || !onGround || justLanded {
			// 1.12.2 使用 Entity Teleport (0x4C) 比较简单可靠
			pkt := entity.BuildEntityTeleport(mob.EID, mob.X, mob.Y, mob.Z, mob.Yaw, mob.Pitch, mob.OnGround)
			for _, p := range players {
				p.Client.Send(pkt)
			}

			// 5. 每隔一段时间更新头部朝向
			if GlobalGameState.tickCount%5 == 0 {
				if mob.VelocityX != 0 || mob.VelocityZ != 0 {
					yaw := float32(math.Atan2(-mob.VelocityX, mob.VelocityZ) * 180 / math.Pi)
					mob.Yaw = yaw
					headPkt := entity.BuildEntityHeadLook(mob.EID, yaw)
					for _, p := range players {
						p.Client.Send(headPkt)
					}
				}
			}
		}
	}
}

// tickPlayerPhysics 单个玩家的物理tick
func tickPlayerPhysics(op *player.OnlinePlayer) {
	pe := op.PlayerEntity
	phys := getOrCreatePhysics(pe.EID)

	x, y, z := pe.X, pe.Y, pe.Z

	// 初始化：第一tick记录初始位置，不施加重力
	if !phys.Initialized {
		phys.LastY = y
		phys.Initialized = true
		phys.OnGround = checkOnGround(x, y, z)
		return
	}

	// === 1. 地面检测 ===
	wasOnGround := phys.OnGround
	onGround := checkOnGround(x, y, z)
	phys.OnGround = onGround

	// === 2. 掉落距离追踪 ===
	deltaY := phys.LastY - y // 正值表示下降

	if deltaY > 0 {
		// 玩家Y在减小（正在下落）
		phys.FallDistance += deltaY
	}

	// === 3. 落地检测（从空中落到地面）===
	if !wasOnGround && onGround && phys.FallDistance > MaxFallDistNoDmg {
		damage := int(math.Floor(phys.FallDistance - MaxFallDistNoDmg))
		applyFallDamage(op, damage)
	}
	if onGround || (wasOnGround && onGround) {
		if wasOnGround != onGround || phys.FallDistance > 0 {
			phys.FallDistance = 0
			phys.Velocity.VY = 0
		}
	}

	phys.LastY = y

	// === 4. 重力模拟及位置修正（已禁用：由客户端主导移动，防止拉回）===
	// 仅在生存/冒险模式下进行逻辑检查，但不强制覆盖位置
	if pe.Gamemode == 1 || pe.Gamemode == 3 {
		return // 创造/旁观不受重力影响
	}

	// 注意：在 Minecraft 中，客户端对自己的移动具有权威性。
	// 服务端只需要跟踪状态（如 OnGround, FallDistance），而不应在每个 Tick 强制施加重力。
	// 强制施加重力并发送位置修正会导致玩家在跳跃时被拉回。

	/*
		// 原有的重力模拟逻辑会导致拉回
		if !onGround {
			phys.Velocity.VY += Gravity
			// ...
			sendPositionCorrection(op)
		}
	*/
}

// ====== 碰撞检测工具函数 ======

// checkOnGround 检测玩家脚底是否站在固体方块上
func checkOnGround(x, y, z float64) bool {
	footY := y - 0.05 // 微低于脚底
	return world.IsBlockSolid(x, footY, z)
}

// ====== 服务端位置修正 ======

// ====== 服务端位置修正 ======

// sendPositionCorrection 将服务端计算的正确位置发给客户端
func sendPositionCorrection(op *player.OnlinePlayer) {
	pe := op.PlayerEntity
	// 修正：向玩家发送自己的位置修正应使用 Player Position and Look (0x2F) 包
	// 这里的 teleportID 暂时设为 0
	pkt := Protocol.BuildAbsoluteTeleport(
		pe.X,
		pe.Y,
		pe.Z,
		pe.Yaw,
		pe.Pitch,
		0,
	)
	op.Client.Send(pkt)
}

// ====== 伤害系统（基础）=====

// applyFallDamage 施加掉落伤害
func applyFallDamage(op *player.OnlinePlayer, damagePoints int) {
	if damagePoints <= 0 {
		return
	}
	fallDist := float64(0)
	physMu.RLock()
	if p, ok := physMap[op.PlayerEntity.EID]; ok {
		fallDist = p.FallDistance
	}
	physMu.RUnlock()

	fmt.Printf("[Tick/Damage] 玩家 %s 掉落伤害: %d 点 (距离: %.1f 格)\n",
		op.Username, damagePoints, fallDist)

	// TODO: 后续接入完整伤害系统时：
	//   1. 发送 Entity Damage 包 (Animation: hurt)
	//   2. 更新生命值 Update Health 包
	//   3. 死亡判定 (health <= 0 -> respawn screen)
	//   4. 击退向量
}


