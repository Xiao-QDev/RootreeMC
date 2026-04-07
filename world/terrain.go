// Package world 游戏世界生成 - 地形与植被系统
package world

import (
	"math"
	"math/bits"
	"sync"
)

// 方块状态定义 (ID << 4 | Metadata)
const (
	BlockAir          uint16 = 0 << 4
	BlockStone        uint16 = 1 << 4
	BlockGrass        uint16 = 2 << 4
	BlockDirt         uint16 = 3 << 4
	BlockBedrock      uint16 = 7 << 4
	BlockWater        uint16 = 9 << 4
	BlockSand         uint16 = 12 << 4
	BlockLogOak       uint16 = (17 << 4) | 0
	BlockLogSpruce    uint16 = (17 << 4) | 1
	BlockLogBirch     uint16 = (17 << 4) | 2
	BlockLogJungle    uint16 = (17 << 4) | 3
	BlockLeavesOak    uint16 = (18 << 4) | 0
	BlockLeavesSpruce uint16 = (18 << 4) | 1
	BlockLeavesBirch  uint16 = (18 << 4) | 2
	BlockLeavesJungle uint16 = (18 << 4) | 3
	TallGrass         uint16 = (31 << 4) | 1
	BlockRose         uint16 = 38 << 4
	BlockDandelion    uint16 = 37 << 4
	BlockGravel       uint16 = 13 << 4
	BlockCoalOre      uint16 = 16 << 4
	BlockIronOre      uint16 = 15 << 4
	BlockGoldOre      uint16 = 14 << 4
	BlockDiamondOre   uint16 = 56 << 4
	BlockSugarCane    uint16 = 83 << 4
	BlockSnow         uint16 = 78 << 4
)

const (
	terrainLayers       = 5
	seaLevel            = 63
	minTerrainHeight    = 5
	maxTerrainHeight    = 250
	treeCellSize        = 4
	defaultTerrainSeed  = int64(20260407)
	macroNormFactor     = 1.0 / 1.5    // 1/(1+1/2)
	detailNormFactor    = 1.0 / 0.4375 // 1/(1/4+1/8+1/16)
	unitFloatNormalizer = 1.0 / (1 << 53)
)

var (
	layerFrequency = [terrainLayers]float64{1.0 / 1024.0, 1.0 / 512.0, 1.0 / 256.0, 1.0 / 128.0, 1.0 / 64.0}
	layerAmplitude = [terrainLayers]float64{1.0, 0.5, 0.25, 0.125, 0.0625}
)

type biomeID uint8

const (
	biomeDeepOcean biomeID = iota
	biomeOcean
	biomeBeach
	biomePlains
	biomeForest
	biomeTaiga
	biomeDesert
	biomeJungle
	biomeMountains
)

// ----- 低开销随机与哈希 -----

type splitMix64 struct {
	state uint64
}

func (s *splitMix64) next() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func hash2DWithSeed(x, z int, seed uint64) uint64 {
	h := seed
	h ^= uint64(int64(x)) * 0x9e3779b185ebca87
	h = bits.RotateLeft64(h, 27)
	h ^= uint64(int64(z)) * 0xc2b2ae3d27d4eb4f
	return mix64(h)
}

func hash3DWithSeed(x, y, z int, seed uint64) uint64 {
	h := seed
	h ^= uint64(int64(x)) * 0x9e3779b185ebca87
	h = bits.RotateLeft64(h, 25)
	h ^= uint64(int64(y)) * 0xd1b54a32d192ed03
	h = bits.RotateLeft64(h, 23)
	h ^= uint64(int64(z)) * 0x94d049bb133111eb
	return mix64(h)
}

func unitFloat01(v uint64) float64 {
	return float64(v>>11) * unitFloatNormalizer
}

func fastFloor(v float64) int {
	i := int(v)
	if v < float64(i) {
		return i - 1
	}
	return i
}

func floorDiv(v, divisor int) int {
	if divisor <= 0 {
		return 0
	}
	q := v / divisor
	r := v % divisor
	if r < 0 {
		q--
	}
	return q
}

func fade(t float64) float64 {
	return t * t * t * (t*(t*6.0-15.0) + 10.0)
}

func lerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

func grad2(hash uint8, x, y float64) float64 {
	h := hash & 7
	u := x
	v := y
	if h >= 4 {
		u, v = y, x
	}
	if h&1 != 0 {
		u = -u
	}
	if h&2 != 0 {
		v = -v
	}
	return u + v
}

// ----- Perlin -----

type PerlinNoise struct {
	p [512]uint8
}

func NewPerlinNoise(seed int64) PerlinNoise {
	var n PerlinNoise
	var base [256]uint8
	for i := 0; i < 256; i++ {
		base[i] = uint8(i)
	}

	rng := splitMix64{state: uint64(seed)}
	for i := 255; i > 0; i-- {
		j := int(rng.next() % uint64(i+1))
		base[i], base[j] = base[j], base[i]
	}

	for i := 0; i < 256; i++ {
		n.p[i] = base[i]
		n.p[256+i] = base[i]
	}
	return n
}

func (n *PerlinNoise) Noise2D(x, y float64) float64 {
	xi := fastFloor(x)
	yi := fastFloor(y)

	xf := x - float64(xi)
	yf := y - float64(yi)

	u := fade(xf)
	v := fade(yf)

	X := xi & 255
	Y := yi & 255

	aa := n.p[int(n.p[X])+Y]
	ab := n.p[int(n.p[X])+Y+1]
	ba := n.p[int(n.p[X+1])+Y]
	bb := n.p[int(n.p[X+1])+Y+1]

	x1 := lerp(u, grad2(aa, xf, yf), grad2(ba, xf-1.0, yf))
	x2 := lerp(u, grad2(ab, xf, yf-1.0), grad2(bb, xf-1.0, yf-1.0))

	return lerp(v, x1, x2)
}

// ----- 地形生成器 -----

type terrainGenerator struct {
	seed     int64
	layers   [terrainLayers]PerlinNoise
	temp     PerlinNoise
	humidity PerlinNoise
}

type terrainNoiseSample struct {
	macro    float64
	detail   float64
}

func newTerrainGenerator(seed int64) *terrainGenerator {
	rng := splitMix64{state: uint64(seed)}
	g := &terrainGenerator{seed: seed}
	for i := 0; i < terrainLayers; i++ {
		g.layers[i] = NewPerlinNoise(int64(rng.next()))
	}
	g.temp = NewPerlinNoise(int64(rng.next()))
	g.humidity = NewPerlinNoise(int64(rng.next()))
	return g
}

func (g *terrainGenerator) sampleFiveLayerNoise(wx, wz int) terrainNoiseSample {
	x := float64(wx)
	z := float64(wz)

	macro := 0.0
	detail := 0.0

	for i := 0; i < terrainLayers; i++ {
		n := g.layers[i].Noise2D(x*layerFrequency[i], z*layerFrequency[i])
		amp := layerAmplitude[i]
		contrib := n * amp
		if i < 2 {
			macro += contrib
		} else {
			detail += contrib
		}
	}

	return terrainNoiseSample{
		macro:    macro * macroNormFactor,
		detail:   detail * detailNormFactor,
	}
}

func (g *terrainGenerator) sampleColumn(wx, wz int) (int, biomeID) {
	ns := g.sampleFiveLayerNoise(wx, wz)
	continental := ns.macro

	// Minecraft 风格：海平面附近 + 大尺度起伏 + 高频细节 + 陆地抬升
	h := float64(seaLevel) + continental*34.0 + ns.detail*8.0

	if continental > 0.16 {
		m := continental - 0.16
		h += m * m * 65.0
	}
	if continental < -0.38 {
		o := -0.38 - continental
		h -= o * o * 90.0
	}

	if h < minTerrainHeight {
		h = minTerrainHeight
	}
	if h > maxTerrainHeight {
		h = maxTerrainHeight
	}

	height := int(h + 0.5)
	x := float64(wx)
	z := float64(wz)
	temp := g.temp.Noise2D(x/900.0, z/900.0)
	hum := g.humidity.Noise2D(x/900.0, z/900.0)

	return height, resolveBiome(height, continental, temp, hum)
}

func resolveBiome(height int, continental, temp, humidity float64) biomeID {
	if continental < -0.55 {
		return biomeDeepOcean
	}
	if continental < -0.25 || height <= seaLevel-4 {
		return biomeOcean
	}
	if height <= seaLevel+1 {
		return biomeBeach
	}
	if height > 132 && continental > 0.25 {
		return biomeMountains
	}
	if temp > 0.45 && humidity < 0.0 {
		return biomeDesert
	}
	if temp > 0.30 && humidity > 0.20 {
		return biomeJungle
	}
	if temp < -0.25 {
		return biomeTaiga
	}
	if humidity > 0.15 {
		return biomeForest
	}
	return biomePlains
}

func biomeName(b biomeID) string {
	switch b {
	case biomeDeepOcean:
		return "Deep Ocean"
	case biomeOcean:
		return "Ocean"
	case biomeBeach:
		return "Beach"
	case biomePlains:
		return "Plains"
	case biomeForest:
		return "Forest"
	case biomeTaiga:
		return "Taiga"
	case biomeDesert:
		return "Desert"
	case biomeJungle:
		return "Jungle"
	case biomeMountains:
		return "Mountains"
	default:
		return "Plains"
	}
}

func surfaceForBiome(b biomeID, height int) (surface uint16, under uint16) {
	surface, under = BlockGrass, BlockDirt

	switch b {
	case biomeDeepOcean, biomeOcean:
		surface, under = BlockSand, BlockSand
	case biomeBeach:
		surface, under = BlockSand, BlockSand
	case biomeDesert:
		surface, under = BlockSand, BlockSand
	case biomeMountains:
		if height > 150 {
			surface, under = BlockSnow, BlockStone
		} else if height > 120 {
			surface, under = BlockGravel, BlockStone
		} else {
			surface, under = BlockStone, BlockStone
		}
	}

	if height > 170 {
		surface, under = BlockSnow, BlockStone
	}
	return
}

func oreOrStone(wx, wz, y int) uint16 {
	h := hash3DWithSeed(wx, y, wz, 0x9f9d2f4b4f6f6f6d)

	switch {
	case y < 16 && (h&0x1FF) == 0: // 1/512
		return BlockDiamondOre
	case y < 32 && (h&0xFF) == 0: // 1/256
		return BlockGoldOre
	case y < 64 && (h&0x7F) == 0: // 1/128
		return BlockIronOre
	case y < 128 && (h&0x3F) == 0: // 1/64
		return BlockCoalOre
	default:
		return BlockStone
	}
}

var (
	terrainMu sync.RWMutex
	terrain   = newTerrainGenerator(defaultTerrainSeed)
)

// SetTerrainSeed 设置地形种子。
func SetTerrainSeed(seed int64) {
	terrainMu.Lock()
	terrain = newTerrainGenerator(seed)
	terrainMu.Unlock()
}

// GetTerrainSeed 返回当前地形种子。
func GetTerrainSeed() int64 {
	terrainMu.RLock()
	defer terrainMu.RUnlock()
	return terrain.seed
}

// GetHeight 返回世界坐标列的地表高度。
func GetHeight(x, z int) int {
	terrainMu.RLock()
	g := terrain
	terrainMu.RUnlock()
	h, _ := g.sampleColumn(x, z)
	return h
}

// GetBiomeName 返回世界坐标的生物群系名称。
func GetBiomeName(x, z int) string {
	terrainMu.RLock()
	g := terrain
	terrainMu.RUnlock()
	_, biome := g.sampleColumn(x, z)
	return biomeName(biome)
}

type treeInfo struct {
	wx, wz, y int
	log, leaf uint16
	style     string
	h         int
}

func treeChanceByBiome(b biomeID) float64 {
	switch b {
	case biomeForest:
		return 0.18
	case biomeTaiga:
		return 0.15
	case biomeJungle:
		return 0.24
	case biomeMountains:
		return 0.06
	default:
		return 0.03
	}
}

// scanTreesInRegion 使用网格候选点扫描，避免逐格随机，提升地形装饰性能。
func scanTreesInRegion(g *terrainGenerator, minX, minZ, maxX, maxZ int) []treeInfo {
	cellMinX := floorDiv(minX, treeCellSize)
	cellMaxX := floorDiv(maxX, treeCellSize)
	cellMinZ := floorDiv(minZ, treeCellSize)
	cellMaxZ := floorDiv(maxZ, treeCellSize)

	trees := make([]treeInfo, 0, 64)
	seedBase := uint64(g.seed) ^ 0x7f4a7c159e3779b9

	for cx := cellMinX; cx <= cellMaxX; cx++ {
		for cz := cellMinZ; cz <= cellMaxZ; cz++ {
			h := hash2DWithSeed(cx, cz, seedBase)
			wx := cx*treeCellSize + int((h>>8)&uint64(treeCellSize-1))
			wz := cz*treeCellSize + int((h>>12)&uint64(treeCellSize-1))

			if wx < minX || wx > maxX || wz < minZ || wz > maxZ {
				continue
			}

			height, biome := g.sampleColumn(wx, wz)
			if height <= seaLevel || height >= 255 {
				continue
			}

			chance := treeChanceByBiome(biome)
			if unitFloat01(h) >= chance {
				continue
			}

			log, leaf, style, th := getTreeConfig(biome, h)
			trees = append(trees, treeInfo{wx: wx, wz: wz, y: height, log: log, leaf: leaf, style: style, h: th})
		}
	}
	return trees
}

func getTreeConfig(b biomeID, h uint64) (uint16, uint16, string, int) {
	rnd := h >> 16
	switch b {
	case biomeTaiga:
		return BlockLogSpruce, BlockLeavesSpruce, "spruce", 6 + int((rnd>>3)&3)
	case biomeJungle:
		return BlockLogJungle, BlockLeavesJungle, "jungle", 10 + int((rnd>>2)&7)
	case biomeForest, biomeMountains:
		if (rnd & 0xFF) < 51 { // 约20%
			return BlockLogBirch, BlockLeavesBirch, "birch", 5 + int((rnd>>8)&3)
		}
		return BlockLogOak, BlockLeavesOak, "oak", 4 + int((rnd>>7)&3)
	default:
		return BlockLogOak, BlockLeavesOak, "oak", 4 + int((rnd>>6)&2)
	}
}

func (chunk *Chunk) GenerateChunk() {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16

	terrainMu.RLock()
	g := terrain
	terrainMu.RUnlock()

	var heights [16][16]int
	var biomes [16][16]biomeID

	// 1. 基础地形（热路径：零分配 + 最少无效循环）
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			wx, wz := worldBaseX+lx, worldBaseZ+lz
			height, biome := g.sampleColumn(wx, wz)
			heights[lx][lz] = height
			biomes[lx][lz] = biome

			surface, under := surfaceForBiome(biome, height)

			chunk.Blocks[lx][0][lz] = BlockBedrock

			stoneTop := height - 4
			if stoneTop < 1 {
				stoneTop = 1
			}
			if stoneTop > 255 {
				stoneTop = 255
			}

			for ly := 1; ly <= stoneTop; ly++ {
				chunk.Blocks[lx][ly][lz] = oreOrStone(wx, wz, ly)
			}

			fillFrom := stoneTop + 1
			if fillFrom < 1 {
				fillFrom = 1
			}
			for ly := fillFrom; ly < height && ly < 256; ly++ {
				chunk.Blocks[lx][ly][lz] = under
			}

			if height > 0 && height < 256 {
				chunk.Blocks[lx][height][lz] = surface
			}

			// 新区块默认就是空气，因此只填海平面以下水体即可。
			if height < seaLevel {
				for ly := height + 1; ly <= seaLevel && ly < 256; ly++ {
					chunk.Blocks[lx][ly][lz] = BlockWater
				}
			}
		}
	}

	// 2. 无接缝植被装饰（扫描边界延伸区域）
	trees := scanTreesInRegion(g, worldBaseX-5, worldBaseZ-5, worldBaseX+20, worldBaseZ+20)
	for _, t := range trees {
		renderTreeInChunk(chunk, t)
	}

	// 3. 小装饰 (草花, 甘蔗)
	seedBase := uint64(g.seed) ^ 0xa24baed4963ee407
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			h := heights[lx][lz]
			if h < seaLevel || h >= 255 {
				continue
			}

			if chunk.Blocks[lx][h+1][lz] != BlockAir {
				continue
			}

			wx, wz := worldBaseX+lx, worldBaseZ+lz
			decoHash := hash2DWithSeed(wx, wz, seedBase)
			roll := unitFloat01(decoHash)
			biome := biomes[lx][lz]

			if biome != biomeOcean && biome != biomeDeepOcean && biome != biomeBeach {
				if roll < 0.07 {
					chunk.Blocks[lx][h+1][lz] = TallGrass
				} else if roll < 0.08 {
					chunk.Blocks[lx][h+1][lz] = BlockDandelion
				} else if roll < 0.09 {
					chunk.Blocks[lx][h+1][lz] = BlockRose
				}
			}

			surface := chunk.Blocks[lx][h][lz]
			if (surface == BlockGrass || surface == BlockSand) && h <= seaLevel+1 {
				if isNearWaterInChunk(chunk, lx, h, lz) {
					caneChance := 0.12
					if biome == biomeJungle {
						caneChance = 0.20
					}
					if unitFloat01(mix64(decoHash^0x6a09e667f3bcc909)) < caneChance {
						caneHeight := 2 + int((decoHash>>48)&1)
						for ch := 1; ch <= caneHeight; ch++ {
							y := h + ch
							if y >= 256 {
								break
							}
							if chunk.Blocks[lx][y][lz] != BlockAir {
								break
							}
							chunk.Blocks[lx][y][lz] = BlockSugarCane
						}
					}
				}
			}
		}
	}
}

func isNearWaterInChunk(chunk *Chunk, lx, y, lz int) bool {
	if lx > 0 && chunk.Blocks[lx-1][y][lz] == BlockWater {
		return true
	}
	if lx < 15 && chunk.Blocks[lx+1][y][lz] == BlockWater {
		return true
	}
	if lz > 0 && chunk.Blocks[lx][y][lz-1] == BlockWater {
		return true
	}
	if lz < 15 && chunk.Blocks[lx][y][lz+1] == BlockWater {
		return true
	}
	return false
}

func renderTreeInChunk(chunk *Chunk, t treeInfo) {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16

	// 渲染树干
	for i := 1; i <= t.h; i++ {
		lx, ly, lz := t.wx-worldBaseX, t.y+i, t.wz-worldBaseZ
		if lx >= 0 && lx < 16 && lz >= 0 && lz < 16 && ly < 256 {
			chunk.Blocks[lx][ly][lz] = t.log
		}
	}

	// 渲染树冠
	switch t.style {
	case "oak", "birch":
		renderOakCanopy(chunk, t)
	case "spruce":
		renderSpruceCanopy(chunk, t)
	case "jungle":
		renderJungleCanopy(chunk, t)
	}
}

func renderOakCanopy(chunk *Chunk, t treeInfo) {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16
	topY := t.y + t.h
	for dy := -2; dy <= 2; dy++ {
		radius := 2
		if dy == 1 {
			radius = 1
		} else if dy == 2 {
			radius = 0
		}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if radius > 1 && absI(dx) == radius && absI(dz) == radius {
					continue
				} // 自然切角
				setBlock(chunk, t.wx+dx-worldBaseX, topY+dy, t.wz+dz-worldBaseZ, t.leaf)
			}
		}
	}
}

func renderSpruceCanopy(chunk *Chunk, t treeInfo) {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16
	topY := t.y + t.h
	radius := 0
	for dy := 1; dy >= -5; dy-- {
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				setBlock(chunk, t.wx+dx-worldBaseX, topY+dy, t.wz+dz-worldBaseZ, t.leaf)
			}
		}
		if radius == 0 {
			radius = 1
		} else if radius == 1 {
			radius = 2
		} else {
			radius--
		}
	}
}

func renderJungleCanopy(chunk *Chunk, t treeInfo) {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16
	topY := t.y + t.h
	for dy := -1; dy <= 2; dy++ {
		radius := 3 - dy
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				if dx*dx+dz*dz <= radius*radius {
					setBlock(chunk, t.wx+dx-worldBaseX, topY+dy, t.wz+dz-worldBaseZ, t.leaf)
				}
			}
		}
	}
}

func setBlock(chunk *Chunk, lx, ly, lz int, state uint16) {
	if lx >= 0 && lx < 16 && lz >= 0 && lz < 16 && ly > 0 && ly < 256 {
		if chunk.Blocks[lx][ly][lz] == BlockAir {
			chunk.Blocks[lx][ly][lz] = state
		}
	}
}

func absI(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// IsBlockSolid 指定位置是否为固体方块
func IsBlockSolid(x, y, z float64) bool {
	bx := int32(math.Floor(x))
	by := int32(math.Floor(y))
	bz := int32(math.Floor(z))

	if by < 0 || by > 255 {
		return false
	}

	block := GlobalWorld.GetBlock(bx, by, bz)
	id := uint16(block >> 4)

	// 简单判断常见非固体方块 (1.12.2)
	switch id {
	case 0: // Air
		return false
	case 8, 9: // Water
		return false
	case 10, 11: // Lava
		return false
	case 31, 32: // Grass, Dead Bush
		return false
	case 37, 38: // Flowers
		return false
	case 6, 39, 40: // Saplings, Mushrooms
		return false
	case 50, 75, 76: // Torches
		return false
	case 51: // Fire
		return false
	case 175: // Large flowers
		return false
	case 78: // Snow Layer (忽略极薄层)
		return false
	default:
		return true
	}
}

// IsOnGround 检查实体是否在地面上
func IsOnGround(x, y, z float64) bool {
	// 检查脚下 0.05 格范围内是否有固体方块
	return IsBlockSolid(x, y-0.05, z)
}
