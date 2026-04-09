// Package world 方块更新系统（液体/火焰）
package world

import (
	"RootreeMC/nbt"
	"fmt"
	"sync"
)

const (
	blockIDAir        uint16 = 0
	blockIDStone      uint16 = 1
	blockIDCobblestone uint16 = 4
	blockIDWaterFlow  uint16 = 8
	blockIDWaterStill uint16 = 9
	blockIDLavaFlow   uint16 = 10
	blockIDLavaStill  uint16 = 11
	blockIDObsidian   uint16 = 49
	blockIDFire       uint16 = 51
	blockIDNetherrack uint16 = 87
)

var (
	neighborDirs = [][3]int32{
		{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1}, {0, 1, 0}, {0, -1, 0},
	}
	horizontalDirs = [][3]int32{
		{1, 0, 0}, {-1, 0, 0}, {0, 0, 1}, {0, 0, -1},
	}
)

// BlockPos 世界坐标
type BlockPos struct {
	X, Y, Z int32
}

// WorldSimulation 管理方块 Tick（液体、火焰）
type WorldSimulation struct {
	mu        sync.Mutex
	worldTick int64
	scheduled map[BlockPos]int64 // 到期 tick
}

// GlobalWorldSimulation 全局方块更新器
var GlobalWorldSimulation = NewWorldSimulation()

// NewWorldSimulation 创建新的方块更新器
func NewWorldSimulation() *WorldSimulation {
	return &WorldSimulation{
		scheduled: make(map[BlockPos]int64),
	}
}

// ScheduleBlockTick 安排一个方块更新
func (ws *WorldSimulation) ScheduleBlockTick(x, y, z int32, delay int64) {
	if y < 0 || y >= 256 {
		return
	}
	if delay < 1 {
		delay = 1
	}

	pos := BlockPos{X: x, Y: y, Z: z}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	due := ws.worldTick + delay
	oldDue, exists := ws.scheduled[pos]
	if !exists || due < oldDue {
		ws.scheduled[pos] = due
	}
}

// OnBlockChanged 在方块变化后安排相关更新
func (ws *WorldSimulation) OnBlockChanged(x, y, z int32, newState uint16) {
	id := GetID(newState)
	if delay := tickDelayForBlock(id); delay > 0 {
		ws.ScheduleBlockTick(x, y, z, delay)
	}

	// 邻居方块可能受影响（如水岩浆反应、火焰蔓延）
	for _, d := range neighborDirs {
		nx, ny, nz := x+d[0], y+d[1], z+d[2]
		if ny < 0 || ny >= 256 {
			continue
		}
		nState := GlobalWorld.GetBlock(nx, ny, nz)
		nID := GetID(nState)
		if delay := tickDelayForBlock(nID); delay > 0 {
			ws.ScheduleBlockTick(nx, ny, nz, delay)
		}
	}
}

// Tick 执行一次方块更新，并返回发生变化的方块列表
func (ws *WorldSimulation) Tick(maxUpdates int) []BlockChange {
	if maxUpdates <= 0 {
		maxUpdates = 256
	}

	ws.mu.Lock()
	ws.worldTick++
	now := ws.worldTick

	duePos := make([]BlockPos, 0, maxUpdates)
	for pos, due := range ws.scheduled {
		if due <= now {
			duePos = append(duePos, pos)
			delete(ws.scheduled, pos)
			if len(duePos) >= maxUpdates {
				break
			}
		}
	}
	ws.mu.Unlock()

	if len(duePos) == 0 {
		return nil
	}

	changed := make(map[BlockPos]uint16, len(duePos)*2)
	for _, pos := range duePos {
		state := GlobalWorld.GetBlock(pos.X, pos.Y, pos.Z)
		id := GetID(state)
		switch {
		case isFluidID(id):
			ws.tickFluid(pos, state, changed)
		case id == blockIDFire:
			ws.tickFire(pos, state, changed, now)
		}
	}

	if len(changed) == 0 {
		return nil
	}

	result := make([]BlockChange, 0, len(changed))
	for pos, state := range changed {
		result = append(result, BlockChange{
			X:       pos.X,
			Y:       pos.Y,
			Z:       pos.Z,
			BlockID: state,
		})
	}
	return result
}

func tickDelayForBlock(id uint16) int64 {
	switch id {
	case blockIDWaterFlow, blockIDWaterStill:
		return 5
	case blockIDLavaFlow, blockIDLavaStill:
		return 30
	case blockIDFire:
		return 10
	default:
		return 0
	}
}

func isFluidID(id uint16) bool {
	return id == blockIDWaterFlow || id == blockIDWaterStill || id == blockIDLavaFlow || id == blockIDLavaStill
}

func isWaterID(id uint16) bool {
	return id == blockIDWaterFlow || id == blockIDWaterStill
}

func isLavaID(id uint16) bool {
	return id == blockIDLavaFlow || id == blockIDLavaStill
}

func fluidFlowID(id uint16) uint16 {
	if isWaterID(id) {
		return blockIDWaterFlow
	}
	return blockIDLavaFlow
}

func fluidStillID(id uint16) uint16 {
	if isWaterID(id) {
		return blockIDWaterStill
	}
	return blockIDLavaStill
}

func isFluidSource(state uint16) bool {
	id := GetID(state)
	if id == blockIDWaterStill || id == blockIDLavaStill {
		return true
	}
	if id == blockIDWaterFlow || id == blockIDLavaFlow {
		return (GetData(state) & 0x7) == 0
	}
	return false
}

func lavaReactionProduct(lavaState uint16, fromAbove bool) uint16 {
	if isFluidSource(lavaState) {
		return ToState(blockIDObsidian, 0)
	}
	if fromAbove {
		return ToState(blockIDStone, 0)
	}
	return ToState(blockIDCobblestone, 0)
}

func isReplaceableByFluid(state uint16) bool {
	id := GetID(state)
	switch id {
	case blockIDAir, blockIDFire:
		return true
	case 31, 32, 37, 38, 39, 40, 50, 75, 76, 78, 175:
		return true
	default:
		return false
	}
}

func isSolidBlockID(id uint16) bool {
	switch id {
	case 0, 8, 9, 10, 11, 31, 32, 37, 38, 39, 40, 50, 51, 75, 76, 78, 175:
		return false
	default:
		return true
	}
}

func isFlammableBlock(id uint16) bool {
	switch id {
	case 5, 17, 18, 31, 32, 35, 47, 53, 54, 58, 63, 64, 65, 68, 85, 96, 107, 126, 134, 135, 136:
		return true
	default:
		return false
	}
}

func flammabilityDivisor(id uint16) uint64 {
	switch id {
	case 18, 31, 32:
		return 2
	case 17, 5, 53, 54, 85, 126, 134, 135, 136:
		return 4
	case 35, 47:
		return 6
	default:
		return 8
	}
}

func (ws *WorldSimulation) randomChance(pos BlockPos, tickNow int64, divisor uint64, salt uint64) bool {
	if divisor <= 1 {
		return true
	}
	h := hash3DWithSeed(
		int(pos.X),
		int(pos.Y+int32(tickNow&0x7FFF)),
		int(pos.Z),
		0x9f9d2f4b4f6f6f6d^salt,
	)
	return h%divisor == 0
}

func (ws *WorldSimulation) setBlockTracked(pos BlockPos, newState uint16, changed map[BlockPos]uint16) bool {
	if pos.Y < 0 || pos.Y >= 256 {
		return false
	}
	oldState := GlobalWorld.GetBlock(pos.X, pos.Y, pos.Z)
	if oldState == newState {
		return false
	}
	if !GlobalWorld.SetBlock(pos.X, pos.Y, pos.Z, newState) {
		return false
	}
	changed[pos] = newState
	return true
}

func (ws *WorldSimulation) tickFluid(pos BlockPos, state uint16, changed map[BlockPos]uint16) {
	id := GetID(state)
	meta := GetData(state) & 0x7
	source := isFluidSource(state)

	if isLavaID(id) && ws.reactLavaAt(pos, state, changed) {
		return
	}

	flowed := false

	// 优先向下流动
	down := BlockPos{X: pos.X, Y: pos.Y - 1, Z: pos.Z}
	if down.Y >= 0 {
		downState := GlobalWorld.GetBlock(down.X, down.Y, down.Z)
		downID := GetID(downState)

		if isWaterID(id) && isLavaID(downID) {
			// 水与下方岩浆反应
			ws.setBlockTracked(down, lavaReactionProduct(downState, true), changed)
			flowed = true
		} else if isLavaID(id) && isWaterID(downID) {
			// 岩浆与下方水反应
			ws.setBlockTracked(pos, lavaReactionProduct(state, true), changed)
			return
		} else if isReplaceableByFluid(downState) {
			ws.setBlockTracked(down, ToState(fluidFlowID(id), 0), changed)
			flowed = true
		}
	}

	// 水平扩散
	if meta < 7 {
		nextLevel := meta + 1
		if source {
			nextLevel = 1
		}

		if nextLevel <= 7 {
			flowState := ToState(fluidFlowID(id), nextLevel)
			for _, d := range horizontalDirs {
				np := BlockPos{X: pos.X + d[0], Y: pos.Y, Z: pos.Z + d[2]}
				nState := GlobalWorld.GetBlock(np.X, np.Y, np.Z)
				nID := GetID(nState)

				if isWaterID(id) && isLavaID(nID) {
					ws.setBlockTracked(np, lavaReactionProduct(nState, false), changed)
					flowed = true
					continue
				}
				if isLavaID(id) && isWaterID(nID) {
					ws.setBlockTracked(pos, lavaReactionProduct(state, false), changed)
					return
				}

				if isReplaceableByFluid(nState) {
					ws.setBlockTracked(np, flowState, changed)
					flowed = true
				}
			}
		}
	}

	// 非源流体在无支撑时逐渐消散
	if !source && !flowed {
		if meta >= 7 {
			ws.setBlockTracked(pos, 0, changed)
		} else {
			ws.setBlockTracked(pos, ToState(fluidFlowID(id), meta+1), changed)
		}
		return
	}

	// 源流体保持静止态（meta=0）
	if source {
		ws.setBlockTracked(pos, ToState(fluidStillID(id), 0), changed)
	}
}

func (ws *WorldSimulation) reactLavaAt(pos BlockPos, state uint16, changed map[BlockPos]uint16) bool {
	down := BlockPos{X: pos.X, Y: pos.Y - 1, Z: pos.Z}
	if down.Y >= 0 {
		downState := GlobalWorld.GetBlock(down.X, down.Y, down.Z)
		if isWaterID(GetID(downState)) {
			ws.setBlockTracked(pos, lavaReactionProduct(state, true), changed)
			return true
		}
	}

	for _, d := range horizontalDirs {
		np := BlockPos{X: pos.X + d[0], Y: pos.Y, Z: pos.Z + d[2]}
		nState := GlobalWorld.GetBlock(np.X, np.Y, np.Z)
		if isWaterID(GetID(nState)) {
			ws.setBlockTracked(pos, lavaReactionProduct(state, false), changed)
			return true
		}
	}
	return false
}

func (ws *WorldSimulation) hasFlammableNeighbor(pos BlockPos) bool {
	for _, d := range neighborDirs {
		np := BlockPos{X: pos.X + d[0], Y: pos.Y + d[1], Z: pos.Z + d[2]}
		if np.Y < 0 || np.Y >= 256 {
			continue
		}
		nState := GlobalWorld.GetBlock(np.X, np.Y, np.Z)
		if isFlammableBlock(GetID(nState)) {
			return true
		}
	}
	return false
}

func (ws *WorldSimulation) tickFire(pos BlockPos, state uint16, changed map[BlockPos]uint16, tickNow int64) {
	age := GetData(state) & 0x0F
	down := BlockPos{X: pos.X, Y: pos.Y - 1, Z: pos.Z}
	downState := uint16(0)
	downID := uint16(0)
	if down.Y >= 0 {
		downState = GlobalWorld.GetBlock(down.X, down.Y, down.Z)
		downID = GetID(downState)
	}
	eternal := downID == blockIDNetherrack

	// 无支撑且无可燃邻居时熄灭
	if !eternal && !isSolidBlockID(downID) && !ws.hasFlammableNeighbor(pos) {
		ws.setBlockTracked(pos, 0, changed)
		return
	}

	// 蔓延点燃可燃方块（简化）
	for _, d := range neighborDirs {
		np := BlockPos{X: pos.X + d[0], Y: pos.Y + d[1], Z: pos.Z + d[2]}
		if np.Y < 0 || np.Y >= 256 {
			continue
		}
		targetState := GlobalWorld.GetBlock(np.X, np.Y, np.Z)
		targetID := GetID(targetState)
		if !isFlammableBlock(targetID) {
			continue
		}

		div := flammabilityDivisor(targetID)
		if ws.randomChance(np, tickNow, div, 0x1234) {
			ws.setBlockTracked(np, ToState(blockIDFire, 0), changed)
		}
	}

	// 老化与熄灭
	if !eternal {
		if age >= 15 {
			if ws.randomChance(pos, tickNow, 2, 0x2234) {
				ws.setBlockTracked(pos, 0, changed)
				return
			}
		} else {
			ws.setBlockTracked(pos, ToState(blockIDFire, age+1), changed)
		}

		if !ws.hasFlammableNeighbor(pos) && ws.randomChance(pos, tickNow, uint64(8+age), 0x3234) {
			ws.setBlockTracked(pos, 0, changed)
			return
		}
	} else if age < 15 && ws.randomChance(pos, tickNow, 3, 0x4234) {
		ws.setBlockTracked(pos, ToState(blockIDFire, age+1), changed)
	}

	_ = downState // 保留变量，后续可用于更细化燃烧规则
}

// ExportPendingTicksNBT 导出待更新队列（可用于世界存档）
func (ws *WorldSimulation) ExportPendingTicksNBT() *nbt.NBT {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	root := nbt.NewCompoundTag()
	root.Set("WorldTick", &nbt.LongTag{Value: ws.worldTick})

	list := nbt.NewListTag(nbt.TagCompound)
	for pos, due := range ws.scheduled {
		entry := nbt.NewCompoundTag()
		entry.Set("x", &nbt.IntTag{Value: pos.X})
		entry.Set("y", &nbt.IntTag{Value: pos.Y})
		entry.Set("z", &nbt.IntTag{Value: pos.Z})
		entry.Set("due", &nbt.LongTag{Value: due})
		list.Append(entry)
	}
	root.Set("ScheduledTicks", list)

	return &nbt.NBT{
		Name: "RootreeMCBlockTicks",
		Root: root,
	}
}

// ImportPendingTicksNBT 导入待更新队列
func (ws *WorldSimulation) ImportPendingTicksNBT(data *nbt.NBT) error {
	if data == nil {
		return nil
	}

	root, ok := data.Root.(*nbt.CompoundTag)
	if !ok {
		return fmt.Errorf("invalid NBT root type: %T", data.Root)
	}

	worldTickTag, ok := root.Get("WorldTick")
	if ok {
		if t, ok := worldTickTag.(*nbt.LongTag); ok {
			ws.mu.Lock()
			ws.worldTick = t.Value
			ws.mu.Unlock()
		}
	}

	listTag, ok := root.Get("ScheduledTicks")
	if !ok {
		return nil
	}
	list, ok := listTag.(*nbt.ListTag)
	if !ok {
		return fmt.Errorf("invalid ScheduledTicks tag type: %T", listTag)
	}

	newMap := make(map[BlockPos]int64, len(list.Value))
	for _, elem := range list.Value {
		entry, ok := elem.(*nbt.CompoundTag)
		if !ok {
			continue
		}
		x, okX := entry.GetInt("x")
		y, okY := entry.GetInt("y")
		z, okZ := entry.GetInt("z")
		dueTag, okDueTag := entry.Get("due")
		if !okX || !okY || !okZ || !okDueTag {
			continue
		}
		due, okDue := dueTag.(*nbt.LongTag)
		if !okDue {
			continue
		}
		if y < 0 || y >= 256 {
			continue
		}
		newMap[BlockPos{X: x, Y: y, Z: z}] = due.Value
	}

	ws.mu.Lock()
	ws.scheduled = newMap
	ws.mu.Unlock()
	return nil
}
