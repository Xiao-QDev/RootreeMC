// Package world 结构生成（原版风格分布 + 轻量模板）
package world

const (
	villageSpacing    = 32
	villageSeparation = 8
	templeSpacing     = 32
	templeSeparation  = 8
)

const (
	stateCobblestone          uint16 = 4 << 4
	stateMossyCobble          uint16 = 48 << 4
	stateOakPlanks            uint16 = 5 << 4
	stateOakLog               uint16 = (17 << 4) | 0
	stateSandstone            uint16 = 24 << 4
	stateSandSmooth           uint16 = (24 << 4) | 2
	stateSandChisel           uint16 = (24 << 4) | 1
	stateSandStairsEast       uint16 = (128 << 4) | 0
	stateSandStairsWest       uint16 = (128 << 4) | 1
	stateSandStairsSouth      uint16 = (128 << 4) | 2
	stateSandStairsNorth      uint16 = (128 << 4) | 3
	stateSandSlab             uint16 = (44 << 4) | 1
	stateOrangeTerracotta     uint16 = (159 << 4) | 1
	stateBlueTerracotta       uint16 = (159 << 4) | 11
	stateFence                uint16 = 85 << 4
	stateLadder               uint16 = (65 << 4) | 2
	stateChest                uint16 = 54 << 4
	stateTorch                uint16 = (50 << 4) | 5
	statePressurePlate        uint16 = 70 << 4
	stateTNT                  uint16 = 46 << 4
	stateStoneBrickChiseled uint16 = (98 << 4) | 3
	stateLever               uint16 = (69 << 4) | 0
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

func getWorldBlockInChunk(chunk *Chunk, wx, y, wz int) (uint16, bool) {
	if y < 0 || y >= 256 {
		return BlockAir, false
	}

	baseX := int(chunk.X) * 16
	baseZ := int(chunk.Z) * 16
	lx := wx - baseX
	lz := wz - baseZ
	if lx < 0 || lx >= 16 || lz < 0 || lz >= 16 {
		return BlockAir, false
	}
	return chunk.Blocks[lx][y][lz], true
}

func isAirOrLiquid(state uint16) bool {
	id := state >> 4
	return id == 0 || id == 8 || id == 9 || id == 10 || id == 11
}

func setRelativeBlockInChunk(chunk *Chunk, originX, originY, originZ, x, y, z int, state uint16) {
	setWorldBlockInChunk(chunk, originX+x, originY+y, originZ+z, state)
}

func fillRelativeBox(chunk *Chunk, originX, originY, originZ, xMin, yMin, zMin, xMax, yMax, zMax int, boundaryState, insideState uint16) {
	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}
	if zMin > zMax {
		zMin, zMax = zMax, zMin
	}

	for y := yMin; y <= yMax; y++ {
		for x := xMin; x <= xMax; x++ {
			for z := zMin; z <= zMax; z++ {
				state := insideState
				if y == yMin || y == yMax || x == xMin || x == xMax || z == zMin || z == zMax {
					state = boundaryState
				}
				setRelativeBlockInChunk(chunk, originX, originY, originZ, x, y, z, state)
			}
		}
	}
}

func fillRelativeSolid(chunk *Chunk, originX, originY, originZ, xMin, yMin, zMin, xMax, yMax, zMax int, state uint16) {
	fillRelativeBox(chunk, originX, originY, originZ, xMin, yMin, zMin, xMax, yMax, zMax, state, state)
}

func replaceAirAndLiquidDownwardsInChunk(chunk *Chunk, originX, originY, originZ, x, y, z int, state uint16) {
	wx := originX + x
	wz := originZ + z
	wy := originY + y

	for wy > 1 {
		current, ok := getWorldBlockInChunk(chunk, wx, wy, wz)
		if !ok || !isAirOrLiquid(current) {
			return
		}
		setWorldBlockInChunk(chunk, wx, wy, wz, state)
		wy--
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

func generateDesertTemple(chunk *Chunk, startX, startZ int) {
	const (
		width  = 21
		depth  = 21
		baseY  = 64
		center = 10
	)

	fillRelativeSolid(chunk, startX, baseY, startZ, 0, -4, 0, width-1, 0, depth-1, stateSandstone)

	for i := 1; i <= 9; i++ {
		fillRelativeSolid(chunk, startX, baseY, startZ, i, i, i, width-1-i, i, depth-1-i, stateSandstone)
		fillRelativeSolid(chunk, startX, baseY, startZ, i+1, i, i+1, width-2-i, i, depth-2-i, BlockAir)
	}

	for x := 0; x < width; x++ {
		for z := 0; z < depth; z++ {
			replaceAirAndLiquidDownwardsInChunk(chunk, startX, baseY, startZ, x, -5, z, stateSandstone)
		}
	}

	fillRelativeBox(chunk, startX, baseY, startZ, 0, 0, 0, 4, 9, 4, stateSandstone, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 1, 10, 1, 3, 10, 3, stateSandstone)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 2, 10, 0, stateSandStairsNorth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 2, 10, 4, stateSandStairsSouth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 0, 10, 2, stateSandStairsEast)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 4, 10, 2, stateSandStairsWest)

	fillRelativeBox(chunk, startX, baseY, startZ, width-5, 0, 0, width-1, 9, 4, stateSandstone, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-4, 10, 1, width-2, 10, 3, stateSandstone)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-3, 10, 0, stateSandStairsNorth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-3, 10, 4, stateSandStairsSouth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-5, 10, 2, stateSandStairsEast)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-1, 10, 2, stateSandStairsWest)

	fillRelativeBox(chunk, startX, baseY, startZ, 8, 0, 0, 12, 4, 4, stateSandstone, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 9, 1, 0, 11, 3, 4, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 1, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 2, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 3, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 3, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 3, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 2, 1, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 1, 1, stateSandSmooth)

	fillRelativeBox(chunk, startX, baseY, startZ, 4, 1, 1, 8, 3, 3, stateSandstone, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 4, 1, 2, 8, 2, 2, BlockAir)
	fillRelativeBox(chunk, startX, baseY, startZ, 12, 1, 1, 16, 3, 3, stateSandstone, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 12, 1, 2, 16, 2, 2, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 5, 4, 5, width-6, 4, depth-6, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, 9, 4, 9, 11, 4, 11, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 8, 1, 8, 8, 3, 8, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 12, 1, 8, 12, 3, 8, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 8, 1, 12, 8, 3, 12, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 12, 1, 12, 12, 3, 12, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 1, 1, 5, 4, 4, 11, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-5, 1, 5, width-2, 4, 11, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, 6, 7, 9, 6, 7, 11, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-7, 7, 9, width-7, 7, 11, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, 5, 5, 9, 5, 7, 11, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-6, 5, 9, width-6, 7, 11, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 5, 5, 10, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 5, 6, 10, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 6, 6, 10, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-6, 5, 10, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-6, 6, 10, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-7, 6, 10, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, 2, 4, 4, 2, 6, 4, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-3, 4, 4, width-3, 6, 4, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 2, 4, 5, stateSandStairsNorth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 2, 3, 4, stateSandStairsNorth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-3, 4, 5, stateSandStairsNorth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-3, 3, 4, stateSandStairsNorth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 1, 1, 3, 2, 2, 3, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-3, 1, 3, width-2, 2, 3, stateSandstone)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 1, 1, 2, stateSandstone)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-2, 1, 2, stateSandstone)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 1, 2, 2, stateSandSlab)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-2, 2, 2, stateSandSlab)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 2, 1, 2, stateSandStairsWest)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-3, 1, 2, stateSandStairsEast)
	fillRelativeSolid(chunk, startX, baseY, startZ, 4, 3, 5, 4, 3, 18, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-5, 3, 5, width-5, 3, 17, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, 3, 1, 5, 4, 2, 16, BlockAir)
	fillRelativeSolid(chunk, startX, baseY, startZ, width-6, 1, 5, width-5, 2, 16, BlockAir)

	for z := 5; z <= 17; z += 2 {
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, 4, 1, z, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, 4, 2, z, stateSandChisel)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-5, 1, z, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, width-5, 2, z, stateSandChisel)
	}

	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 0, 7, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 0, 8, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 0, 9, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 0, 9, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 8, 0, 10, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 12, 0, 10, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 7, 0, 10, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 13, 0, 10, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 0, 11, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 0, 11, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 0, 12, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 0, 13, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 0, 10, stateBlueTerracotta)

	for x := 0; x <= width-1; x += width - 1 {
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 2, 1, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 2, 2, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 2, 3, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 3, 1, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 3, 2, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 3, 3, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 4, 1, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 4, 2, stateSandChisel)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 4, 3, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 5, 1, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 5, 2, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 5, 3, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 6, 1, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 6, 2, stateSandChisel)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 6, 3, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 7, 1, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 7, 2, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 7, 3, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 8, 1, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 8, 2, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 8, 3, stateSandSmooth)
	}

	for x := 2; x <= width-3; x += width - 5 {
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 2, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 2, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 2, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 3, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 3, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 3, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 4, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 4, 0, stateSandChisel)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 4, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 5, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 5, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 5, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 6, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 6, 0, stateSandChisel)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 6, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 7, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 7, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 7, 0, stateOrangeTerracotta)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x-1, 8, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x, 8, 0, stateSandSmooth)
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, x+1, 8, 0, stateSandSmooth)
	}

	fillRelativeSolid(chunk, startX, baseY, startZ, 8, 4, 0, 12, 6, 0, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 8, 6, 0, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 12, 6, 0, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 9, 5, 0, stateOrangeTerracotta)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 10, 5, 0, stateSandChisel)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 11, 5, 0, stateOrangeTerracotta)

	fillRelativeSolid(chunk, startX, baseY, startZ, 8, -14, 8, 12, -11, 12, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 8, -10, 8, 12, -10, 12, stateSandChisel)
	fillRelativeSolid(chunk, startX, baseY, startZ, 8, -9, 8, 12, -9, 12, stateSandSmooth)
	fillRelativeSolid(chunk, startX, baseY, startZ, 8, -8, 8, 12, -1, 12, stateSandstone)
	fillRelativeSolid(chunk, startX, baseY, startZ, 9, -11, 9, 11, -1, 11, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -11, center, statePressurePlate)
	fillRelativeBox(chunk, startX, baseY, startZ, 9, -13, 9, 11, -13, 11, stateTNT, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 8, -11, center, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 8, -10, center, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 7, -10, center, stateSandChisel)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 7, -11, center, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 12, -11, center, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 12, -10, center, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 13, -10, center, stateSandChisel)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, 13, -11, center, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -11, 8, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -10, 8, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -10, 7, stateSandChisel)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -11, 7, stateSandSmooth)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -11, 12, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -10, 12, BlockAir)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -10, 13, stateSandChisel)
	setRelativeBlockInChunk(chunk, startX, baseY, startZ, center, -11, 13, stateSandSmooth)

	chestOffsets := [][2]int{{-2, 0}, {2, 0}, {0, -2}, {0, 2}}
	for _, off := range chestOffsets {
		setRelativeBlockInChunk(chunk, startX, baseY, startZ, center+off[0], -11, center+off[1], stateChest)
	}
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
			if biome != biomePlains && biome != biomeSunflowerPlains && biome != biomeDesert && biome != biomeTaiga {
				continue
			}
			generateVillageWell(chunk, centerX, centerZ, height+1, biome)
		}
	}

	// 神殿：原版风格起始点分布（chunk 锚点 + 1.12.2 沙漠神殿结构）
	for sx := chunkX - 1; sx <= chunkX+1; sx++ {
		for sz := chunkZ - 1; sz <= chunkZ+1; sz++ {
			startX := int32(sx)
			startZ := int32(sz)
			if !shouldStartTemple(startX, startZ, seed) {
				continue
			}

			structureStartX := sx * 16
			structureStartZ := sz * 16
			probeX := structureStartX + 8
			probeZ := structureStartZ + 8
			height, biome := g.sampleColumn(probeX, probeZ)
			switch biome {
			case biomeDesert:
				generateDesertTemple(chunk, structureStartX, structureStartZ)
			case biomeJungle:
				generateJungleTemple(chunk, probeX, probeZ, height+1)
			}
		}
	}

	// 废弃矿井暂不写入地表模板，避免出现非原版木框入口。
}
