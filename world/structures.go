// Package world 结构生成（原版风格分布 + 轻量模板）
package world

const (
	villageSpacing    = 32
	villageSeparation = 8
	templeSpacing     = 32
	templeSeparation  = 8
)

const (
	stateCobblestone uint16 = 4 << 4
	stateMossyCobble uint16 = 48 << 4
	stateOakPlanks   uint16 = 5 << 4
	stateOakLog      uint16 = (17 << 4) | 0
	stateSandstone   uint16 = 24 << 4
	stateFence       uint16 = 85 << 4
	stateLadder      uint16 = (65 << 4) | 2
	stateChest       uint16 = 54 << 4
	stateTorch       uint16 = (50 << 4) | 5
)

type javaRandom struct {
	seed int64
}

const (
	javaMultiplier int64 = 0x5DEECE66D
	javaAddend     int64 = 0xB
	javaMask       int64 = (1 << 48) - 1
)

func newJavaRandom(seed int64) *javaRandom {
	r := &javaRandom{}
	r.setSeed(seed)
	return r
}

func (r *javaRandom) setSeed(seed int64) {
	r.seed = (seed ^ javaMultiplier) & javaMask
}

func (r *javaRandom) next(bits int32) int32 {
	r.seed = (r.seed*javaMultiplier + javaAddend) & javaMask
	return int32(uint64(r.seed) >> uint(48-bits))
}

func (r *javaRandom) Intn(bound int32) int32 {
	if bound <= 0 {
		return 0
	}
	if bound&(bound-1) == 0 {
		return int32((int64(bound) * int64(r.next(31))) >> 31)
	}
	for {
		bits := r.next(31)
		val := bits % bound
		if bits-val+(bound-1) >= 0 {
			return val
		}
	}
}

func (r *javaRandom) Float64() float64 {
	hi := int64(r.next(26))
	lo := int64(r.next(27))
	return float64((hi<<27)+lo) / (1 << 53)
}

func shouldStartVillage(chunkX, chunkZ int32, seed int64) bool {
	regionX := floorDiv(int(chunkX), villageSpacing)
	regionZ := floorDiv(int(chunkZ), villageSpacing)

	r := newJavaRandom(int64(regionX)*341873128712 + int64(regionZ)*132897987541 + seed + 10387312)
	candidateX := int32(regionX*villageSpacing + int(r.Intn(villageSpacing-villageSeparation)))
	candidateZ := int32(regionZ*villageSpacing + int(r.Intn(villageSpacing-villageSeparation)))

	return candidateX == chunkX && candidateZ == chunkZ
}

func shouldStartTemple(chunkX, chunkZ int32, seed int64) bool {
	regionX := floorDiv(int(chunkX), templeSpacing)
	regionZ := floorDiv(int(chunkZ), templeSpacing)

	r := newJavaRandom(int64(regionX)*341873128712 + int64(regionZ)*132897987541 + seed + 14357617)
	candidateX := int32(regionX*templeSpacing + int(r.Intn(templeSpacing-templeSeparation)))
	candidateZ := int32(regionZ*templeSpacing + int(r.Intn(templeSpacing-templeSeparation)))

	return candidateX == chunkX && candidateZ == chunkZ
}

func shouldStartMineshaft(chunkX, chunkZ int32, seed int64) bool {
	r := newJavaRandom(int64(chunkX)*341873128712 + int64(chunkZ)*132897987541 + seed + 0x4f9939f508)
	if r.Float64() >= 0.004 {
		return false
	}

	dist := absI(int(chunkX))
	if zAbs := absI(int(chunkZ)); zAbs > dist {
		dist = zAbs
	}
	return int(r.Intn(80)) < dist
}

func setWorldBlockInChunk(chunk *Chunk, wx, y, wz int, state uint16) {
	if y < 0 || y >= 256 {
		return
	}

	baseX := int(chunk.X) * 16
	baseZ := int(chunk.Z) * 16
	lx := wx - baseX
	lz := wz - baseZ
	if lx < 0 || lx >= 16 || lz < 0 || lz >= 16 {
		return
	}
	chunk.Blocks[lx][y][lz] = state
}

func setWorldBlockInChunkIfAir(chunk *Chunk, wx, y, wz int, state uint16) {
	if y < 0 || y >= 256 {
		return
	}

	baseX := int(chunk.X) * 16
	baseZ := int(chunk.Z) * 16
	lx := wx - baseX
	lz := wz - baseZ
	if lx < 0 || lx >= 16 || lz < 0 || lz >= 16 {
		return
	}
	if chunk.Blocks[lx][y][lz] == BlockAir {
		chunk.Blocks[lx][y][lz] = state
	}
}

func generateVillageWell(chunk *Chunk, centerX, centerZ, baseY int, biome biomeID) {
	wallState := stateCobblestone
	roofState := stateOakPlanks
	postState := stateOakLog

	if biome == biomeDesert {
		wallState = stateSandstone
		roofState = stateSandstone
		postState = stateSandstone
	}

	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			wx := centerX + dx
			wz := centerZ + dz

			setWorldBlockInChunk(chunk, wx, baseY-1, wz, wallState)
			if absI(dx) == 2 || absI(dz) == 2 {
				setWorldBlockInChunk(chunk, wx, baseY, wz, wallState)
			} else {
				setWorldBlockInChunk(chunk, wx, baseY, wz, BlockWater)
			}
		}
	}

	postOffsets := [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
	for _, off := range postOffsets {
		for y := baseY + 1; y <= baseY+3; y++ {
			setWorldBlockInChunk(chunk, centerX+off[0], y, centerZ+off[1], postState)
		}
	}

	for dx := -2; dx <= 2; dx++ {
		for dz := -2; dz <= 2; dz++ {
			setWorldBlockInChunk(chunk, centerX+dx, baseY+4, centerZ+dz, roofState)
		}
	}
}

func generateDesertTemple(chunk *Chunk, centerX, centerZ, baseY int) {
	for dx := -4; dx <= 4; dx++ {
		for dz := -4; dz <= 4; dz++ {
			wx := centerX + dx
			wz := centerZ + dz
			setWorldBlockInChunk(chunk, wx, baseY-1, wz, stateSandstone)

			for dy := 0; dy <= 5; dy++ {
				border := absI(dx) == 4 || absI(dz) == 4
				if dy == 0 || border {
					setWorldBlockInChunk(chunk, wx, baseY+dy, wz, stateSandstone)
				} else {
					setWorldBlockInChunk(chunk, wx, baseY+dy, wz, BlockAir)
				}
			}
		}
	}

	orangeWool := ToState(35, 1)
	setWorldBlockInChunk(chunk, centerX, baseY, centerZ, orangeWool)
	setWorldBlockInChunkIfAir(chunk, centerX, baseY+1, centerZ, stateChest)
}

func generateJungleTemple(chunk *Chunk, centerX, centerZ, baseY int) {
	for dx := -4; dx <= 4; dx++ {
		for dz := -4; dz <= 4; dz++ {
			wx := centerX + dx
			wz := centerZ + dz
			for dy := 0; dy <= 5; dy++ {
				border := absI(dx) == 4 || absI(dz) == 4 || dy == 0 || dy == 5
				if border {
					state := stateMossyCobble
					if (dx+dz+dy)&1 == 0 {
						state = stateCobblestone
					}
					setWorldBlockInChunk(chunk, wx, baseY+dy, wz, state)
				} else {
					setWorldBlockInChunk(chunk, wx, baseY+dy, wz, BlockAir)
				}
			}
		}
	}

	setWorldBlockInChunkIfAir(chunk, centerX, baseY+1, centerZ, stateChest)
}

func generateMineshaftEntrance(chunk *Chunk, centerX, centerZ, baseY int) {
	if baseY < 12 || baseY > 220 {
		return
	}

	for dx := -1; dx <= 1; dx++ {
		for dz := -1; dz <= 1; dz++ {
			setWorldBlockInChunk(chunk, centerX+dx, baseY, centerZ+dz, stateOakPlanks)
			setWorldBlockInChunk(chunk, centerX+dx, baseY+4, centerZ+dz, stateOakPlanks)
		}
	}

	postOffsets := [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
	for _, off := range postOffsets {
		for dy := 1; dy <= 3; dy++ {
			setWorldBlockInChunk(chunk, centerX+off[0], baseY+dy, centerZ+off[1], stateOakLog)
		}
	}

	setWorldBlockInChunk(chunk, centerX, baseY+3, centerZ-1, stateTorch)
	setWorldBlockInChunk(chunk, centerX, baseY+3, centerZ+1, stateTorch)
	setWorldBlockInChunk(chunk, centerX-1, baseY+3, centerZ, stateTorch)
	setWorldBlockInChunk(chunk, centerX+1, baseY+3, centerZ, stateTorch)

	for depth := 1; depth <= 6; depth++ {
		y := baseY - depth
		setWorldBlockInChunk(chunk, centerX, y, centerZ, BlockAir)
		setWorldBlockInChunk(chunk, centerX, y, centerZ+1, BlockAir)
		setWorldBlockInChunk(chunk, centerX, y, centerZ-1, BlockAir)
		setWorldBlockInChunk(chunk, centerX, y, centerZ, stateLadder)
	}

	for dz := -1; dz <= 1; dz++ {
		setWorldBlockInChunk(chunk, centerX, baseY-6, centerZ+dz, stateFence)
	}
}

func generateStructuresInChunk(chunk *Chunk, g *terrainGenerator) {
	seed := g.seed
	chunkX := int(chunk.X)
	chunkZ := int(chunk.Z)

	// 村庄：原版风格起始点分布 + 轻量村庄井模板
	for sx := chunkX - 2; sx <= chunkX+2; sx++ {
		for sz := chunkZ - 2; sz <= chunkZ+2; sz++ {
			startX := int32(sx)
			startZ := int32(sz)
			if !shouldStartVillage(startX, startZ, seed) {
				continue
			}

			centerX := sx*16 + 8
			centerZ := sz*16 + 8
			height, biome := g.sampleColumn(centerX, centerZ)
			if biome != biomePlains && biome != biomeDesert && biome != biomeTaiga {
				continue
			}
			generateVillageWell(chunk, centerX, centerZ, height+1, biome)
		}
	}

	// 神殿：原版风格起始点分布（沙漠神殿/丛林神庙简化版）
	for sx := chunkX - 1; sx <= chunkX+1; sx++ {
		for sz := chunkZ - 1; sz <= chunkZ+1; sz++ {
			startX := int32(sx)
			startZ := int32(sz)
			if !shouldStartTemple(startX, startZ, seed) {
				continue
			}

			centerX := sx*16 + 8
			centerZ := sz*16 + 8
			height, biome := g.sampleColumn(centerX, centerZ)
			switch biome {
			case biomeDesert:
				generateDesertTemple(chunk, centerX, centerZ, height+1)
			case biomeJungle:
				generateJungleTemple(chunk, centerX, centerZ, height+1)
			}
		}
	}

	// 废弃矿井：原版风格概率分布（入口简化版）
	if shouldStartMineshaft(chunk.X, chunk.Z, seed) {
		centerX := int(chunk.X)*16 + 8
		centerZ := int(chunk.Z)*16 + 8
		height, _ := g.sampleColumn(centerX, centerZ)
		generateMineshaftEntrance(chunk, centerX, centerZ, height+1)
	}
}
