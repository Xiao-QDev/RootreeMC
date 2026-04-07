// Package world 游戏世界生成 - 地形与植被系统
package world

import (
	"math"
	"math/rand"
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

//噪声工具

type PerlinNoise struct {
	p [512]int
}

func NewPerlinNoise(seed int64) *PerlinNoise {
	n := &PerlinNoise{}
	for i := 0; i < 256; i++ {
		n.p[i] = i
	}
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(256, func(i, j int) { n.p[i], n.p[j] = n.p[j], n.p[i] })
	for i := 0; i < 256; i++ {
		n.p[256+i] = n.p[i]
	}
	return n
}

func (n *PerlinNoise) Noise2D(x, y float64) float64 {
	ix := int(math.Floor(x))
	iy := int(math.Floor(y))
	xf := x - float64(ix)
	yf := y - float64(iy)
	X := ix & 255
	Y := iy & 255

	u := xf * xf * xf * (xf*(xf*6-15) + 10)
	v := yf * yf * yf * (yf*(yf*6-15) + 10)

	A := n.p[X] + Y
	B := n.p[X+1] + Y

	g00 := grad2D(n.p[A], xf, yf)
	g10 := grad2D(n.p[B], xf-1, yf)
	g01 := grad2D(n.p[A+1], xf, yf-1)
	g11 := grad2D(n.p[B+1], xf-1, yf-1)

	lx1 := g00 + u*(g10-g00)
	lx2 := g01 + u*(g11-g01)

	return lx1 + v*(lx2-lx1)
}

func grad2D(h int, x, y float64) float64 {
	switch h & 7 {
	case 0:
		return x + y
	case 1:
		return -x + y
	case 2:
		return x - y
	case 3:
		return -x - y
	case 4:
		return x
	case 5:
		return -x
	case 6:
		return y
	case 7:
		return -y
	}
	return 0
}

func (n *PerlinNoise) FBM(x, y float64, octaves int) float64 {
	total := 0.0
	freq := 1.0
	amp := 1.0
	maxAmp := 0.0
	for i := 0; i < octaves; i++ {
		total += n.Noise2D(x*freq, y*freq) * amp
		maxAmp += amp
		amp *= 0.5
		freq *= 2.0
	}
	return total / maxAmp
}

//地形数据

var (
	contNoise    = NewPerlinNoise(1234) // 大陆性噪声 (海陆分布)
	erosionNoise = NewPerlinNoise(5678) // 侵蚀噪声 (平原/陡峭)
	pvNoise      = NewPerlinNoise(9012) // 峰谷噪声 (山脉形态)
	warpNoise    = NewPerlinNoise(3456) // 扭曲噪声 (增加自然感)
	biomeNoise   = NewPerlinNoise(7890) // 生物群系噪声
)

// 样条曲线控制点
type SplinePoint struct {
	X, Y float64
}

// SampleSpline 在样条曲线上进行线性插值，确保地形连续性
func SampleSpline(points []SplinePoint, x float64) float64 {
	if len(points) == 0 { return 0 }
	if x <= points[0].X { return points[0].Y }
	if x >= points[len(points)-1].X { return points[len(points)-1].Y }

	for i := 0; i < len(points)-1; i++ {
		if x <= points[i+1].X {
			t := (x - points[i].X) / (points[i+1].X - points[i].X)
			return points[i].Y + t*(points[i+1].Y-points[i].Y)
		}
	}
	return points[len(points)-1].Y
}

// GetHeight 使用连续样条曲线生成高度，消除地形断层
func GetHeight(x, z int) int {
	fx, fz := float64(x), float64(z)

	// 1. 基础指标采样
	cont := contNoise.FBM(fx/600, fz/600, 4)
	ero := erosionNoise.FBM(fx/300, fz/300, 3)
	
	// 领域扭曲处理山脉
	wx := fx + warpNoise.Noise2D(fx/100, fz/100)*30.0
	wz := fz + warpNoise.Noise2D(fz/100, fx/100)*30.0
	pv := 1.0 - math.Abs(pvNoise.FBM(wx/150, wz/150, 4))
	pv = pv * pv 

	// 2. 大陆性 -> 基础高度 (消除 if-else 断层)
	contSpline := []SplinePoint{
		{-1.0, 30.0},  // 深海
		{-0.2, 58.0},  // 浅海
		{-0.1, 62.0},  // 海岸
		{0.1, 68.0},   // 平原
		{0.5, 85.0},   // 高地
		{1.0, 110.0},  // 基础高山
	}
	baseHeight := SampleSpline(contSpline, cont)

	// 3. 侵蚀度与峰谷结合 -> 动态山脉加成
	// 只有在大陆性较高且侵蚀度较低时，山脉才会挺拔
	mtnWeight := SampleSpline([]SplinePoint{
		{0.0, 0.0}, {0.2, 1.0}, // 只有大陆性 > 0.2 才开始有山
	}, cont)
	
	steepness := SampleSpline([]SplinePoint{
		{-1.0, 1.0}, {0.1, 0.8}, {0.3, 0.0}, // 侵蚀度 > 0.3 后基本变平原
	}, ero)

	mtnBonus := pv * 80.0 * mtnWeight * steepness
	
	h := baseHeight + mtnBonus

	// 最终约束
	if h < 5 { h = 5 }
	if h > 250 { h = 250 }
	
	return int(h)
}

func GetBiomeName(x, z int) string {
	fx, fz := float64(x), float64(z)
	
	// 使用多层指标确定生物群系
	temp := biomeNoise.Noise2D(fx/600, fz/600)
	cont := contNoise.FBM(fx/600, fz/600, 4)
	ero := erosionNoise.FBM(fx/300, fz/300, 3)

	if cont < -0.2 {
		return "Deep Ocean"
	}
	if cont < 0 {
		return "Ocean"
	}
	
	// 高地山脉判断
	if cont > 0.4 && ero < 0.1 {
		return "Mountains"
	}

	// 气候判断
	if temp > 0.35 {
		return "Desert"
	}
	if temp < -0.25 {
		return "Taiga"
	}
	if temp > 0.15 {
		return "Jungle"
	}
	return "Forest"
}

type treeInfo struct {
	wx, wz, y int
	log, leaf uint16
	style     string
	h         int
}

// scanTreesInRegion 扫描区域内的所有树
func scanTreesInRegion(minX, minZ, maxX, maxZ int) []treeInfo {
	var trees []treeInfo
	// 性能优化: 减少内存分配
	for wx := minX; wx <= maxX; wx++ {
		for wz := minZ; wz <= maxZ; wz++ {
			// 使用更轻量级的哈希
			hVal := (int64(wx)*31337 + int64(wz)*7919)
			if hVal%10 != 0 { continue } // 快速过滤

			rng := rand.New(rand.NewSource(hVal))
			biomeName := GetBiomeName(wx, wz)
			
			// 树木密度控制
			chance := 0.002
			if biomeName == "Forest" {
				chance = 0.015 + 0.01 * contNoise.Noise2D(float64(wx)/50, float64(wz)/50) // 加入密度噪声
			} else if biomeName == "Taiga" {
				chance = 0.01 + 0.01 * contNoise.Noise2D(float64(wx)/80, float64(wz)/80)
			} else if biomeName == "Jungle" {
				chance = 0.02 + 0.02 * contNoise.Noise2D(float64(wx)/30, float64(wz)/30)
			} else if biomeName == "Mountains" {
				chance = 0.005
			}

			height := GetHeight(wx, wz)
			if height >= 63 && rng.Float64() < chance {
				log, leaf, style, h := getTreeConfig(biomeName, rng)
				trees = append(trees, treeInfo{wx, wz, height, log, leaf, style, h})
			}
		}
	}
	return trees
}

func getTreeConfig(biome string, rng *rand.Rand) (uint16, uint16, string, int) {
	switch biome {
	case "Taiga":
		return BlockLogSpruce, BlockLeavesSpruce, "spruce", 6 + rng.Intn(4)
	case "Jungle":
		return BlockLogJungle, BlockLeavesJungle, "jungle", 10 + rng.Intn(8)
	case "Forest", "Mountains":
		r := rng.Float32()
		if r < 0.2 {
			return BlockLogBirch, BlockLeavesBirch, "birch", 5 + rng.Intn(3)
		}
		return BlockLogOak, BlockLeavesOak, "oak", 4 + rng.Intn(3)
	default:
		return BlockLogOak, BlockLeavesOak, "oak", 4 + rng.Intn(2)
	}
}

func (chunk *Chunk) GenerateChunk() {
	worldBaseX, worldBaseZ := int(chunk.X)*16, int(chunk.Z)*16

	// 1. 生成基础地形
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			wx, wz := worldBaseX+lx, worldBaseZ+lz
			height := GetHeight(wx, wz)
			biome := GetBiomeName(wx, wz)

			surface, under := BlockGrass, BlockDirt
			switch biome {
			case "Desert":
				surface, under = BlockSand, BlockSand
			case "Ocean", "Deep Ocean":
				surface, under = BlockSand, BlockDirt
			case "Mountains":
				if height > 140 {
					surface = BlockSnow
				} else if height > 110 {
					surface = BlockGravel
				} else {
					surface = BlockStone
				}
				under = BlockStone
			}
			
			// 高海拔地区即便是森林也可能覆盖积雪
			if height > 160 {
				surface = BlockSnow
				under = BlockStone
			}

			// 使用区间填充优化，避免逐块 switch
			// 0: 基岩
			chunk.Blocks[lx][0][lz] = BlockBedrock

			// 1 -> height-4: 石头
			for ly := 1; ly <= height-4; ly++ {
				state := BlockStone
				// 矿石随机分布 (基于坐标的哈希)
				oreSeed := int64(wx)*37 + int64(wz)*13 + int64(ly)*7
				if (oreSeed % 100) == 0 {
					if ly < 16 && (oreSeed%1000) == 7 {
						state = BlockDiamondOre
					} else if ly < 32 && (oreSeed%200) == 3 {
						state = BlockGoldOre
					} else if (oreSeed % 100) == 0 {
						if (oreSeed % 300) < 100 {
							state = BlockIronOre
						} else {
							state = BlockCoalOre
						}
					}
				}
				chunk.Blocks[lx][ly][lz] = state
			}

			// height-3 -> height-1: 泥土/沙子
			for ly := maxI(1, height-3); ly < height; ly++ {
				chunk.Blocks[lx][ly][lz] = under
			}

			// height: 草方块/沙子
			if height > 0 {
				chunk.Blocks[lx][height][lz] = surface
			}

			// height+1 -> 255: 空气或水
			for ly := height + 1; ly < 256; ly++ {
				chunk.Blocks[lx][ly][lz] = selectWaterOrAir(ly)
			}
		}
	}

	// 2. 无接缝植被装饰 (核心修复：扫描邻近区域树木)
	// 扫描范围只需覆盖能触及该区块的最大树冠半径即可 (通常为 4-5 块)
	trees := scanTreesInRegion(worldBaseX-5, worldBaseZ-5, worldBaseX+20, worldBaseZ+20)
	for _, t := range trees {
		renderTreeInChunk(chunk, t)
	}

	// 3. 小装饰 (草花, 甘蔗)
	for lx := 0; lx < 16; lx++ {
		for lz := 0; lz < 16; lz++ {
			wx, wz := worldBaseX+lx, worldBaseZ+lz
			h := GetHeight(wx, wz)
			if h >= 63 && h < 255 {
				rng := rand.New(rand.NewSource(int64(wx)*13 + int64(wz)*7))
				r := rng.Float64()
				
				// 只有在空气且下方是非空气时放置装饰
				if chunk.Blocks[lx][h+1][lz] == BlockAir {
					if r < 0.08 {
						chunk.Blocks[lx][h+1][lz] = TallGrass
					} else if r < 0.09 {
						chunk.Blocks[lx][h+1][lz] = BlockDandelion
					} else if r < 0.10 {
						chunk.Blocks[lx][h+1][lz] = BlockRose
					}
				}
				
				// 在水边生成甘蔗
				if (chunk.Blocks[lx][h][lz] == BlockGrass || chunk.Blocks[lx][h][lz] == BlockSand) && h <= 64 {
					isNearWater := false
					// 简单检测四周是否有水
					if (lx > 0 && chunk.Blocks[lx-1][h][lz] == BlockWater) || 
					   (lx < 15 && chunk.Blocks[lx+1][h][lz] == BlockWater) ||
					   (lz > 0 && chunk.Blocks[lx][h][lz-1] == BlockWater) ||
					   (lz < 15 && chunk.Blocks[lx][h][lz+1] == BlockWater) {
						isNearWater = true
					}
					if isNearWater && r < 0.15 {
						caneH := 2 + rng.Intn(2)
						for ch := 1; ch <= caneH; ch++ {
							if h+ch < 256 {
								chunk.Blocks[lx][h+ch][lz] = BlockSugarCane
							}
						}
					}
				}
			}
		}
	}
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
			radius -= 1
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

func selectWaterOrAir(y int) uint16 {
	if y <= 63 {
		return BlockWater
	}
	return BlockAir
}

func absI(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
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
