// Package inventory 玩家物品栏系统
package inventory

import (
	"sync"
)

// ItemStack 物品堆叠
type ItemStack struct {
	ItemID   int32 // 物品ID
	Count    byte  // 数量
	Damage   int16 // 耐久/元数据
	HasNBT   bool  // 是否有NBT
	NBTData  []byte // NBT数据（简化）
}

// Inventory 玩家物品栏（1.12.2）
// 0-8: 快捷栏
// 9-35: 背包
// 36-39: 盔甲
// 40: 副手 (1.9+)
type Inventory struct {
	Slots [41]ItemStack // 1.12.2 总共41个槽位
	mu    sync.Mutex
}

// NewInventory 创建新物品栏
func NewInventory() *Inventory {
	inv := &Inventory{}
	// 初始化空槽位
	for i := range inv.Slots {
		inv.Slots[i] = ItemStack{} // 空物品
	}
	return inv
}

// SetItem 设置物品到指定槽位
func (inv *Inventory) SetItem(slot int, item ItemStack) bool {
	if slot < 0 || slot >= len(inv.Slots) {
		return false
	}
	
	inv.mu.Lock()
	defer inv.mu.Unlock()
	
	inv.Slots[slot] = item
	return true
}

// GetItem 获取槽位中的物品
func (inv *Inventory) GetItem(slot int) (ItemStack, bool) {
	if slot < 0 || slot >= len(inv.Slots) {
		return ItemStack{}, false
	}
	
	inv.mu.Lock()
	defer inv.mu.Unlock()
	
	return inv.Slots[slot], true
}

// GetHeldItem 获取手持物品（快捷栏选中项）
func (inv *Inventory) GetHeldItem(hotbarSlot int) ItemStack {
	if hotbarSlot < 0 || hotbarSlot > 8 {
		return ItemStack{}
	}
	
	item, _ := inv.GetItem(hotbarSlot)
	return item
}

// AddItem 添加物品到物品栏（自动寻找空位）
func (inv *Inventory) AddItem(item ItemStack) bool {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	
	// 优先合并到相同物品的槽位
	for i := 0; i < 36; i++ {
		slot := &inv.Slots[i]
		if slot.ItemID == item.ItemID && slot.Count < 64 {
			available := 64 - slot.Count
			if int(item.Count) <= int(available) {
				slot.Count += item.Count
				return true
			}
		}
	}
	
	// 寻找空槽位
	for i := 0; i < 36; i++ {
		if inv.Slots[i].ItemID == 0 {
			inv.Slots[i] = item
			return true
		}
	}
	
	return false // 物品栏已满
}

// Clear 清空物品栏
func (inv *Inventory) Clear() {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	
	for i := range inv.Slots {
		inv.Slots[i] = ItemStack{}
	}
}

// 常见物品ID（Minecraft 1.12.2）
const (
	ItemAir          = 0
	ItemStone        = 1
	ItemGrass        = 2
	ItemDirt         = 3
	ItemCobblestone  = 4
	ItemOakPlanks    = 5
	ItemOakSapling   = 6
	ItemBedrock      = 7
	ItemWater        = 8
	ItemLava         = 11
	ItemSand         = 12
	ItemGravel       = 13
	ItemGoldOre      = 14
	ItemIronOre      = 15
	ItemCoalOre      = 16
	ItemOakLog       = 17
	ItemOakLeaves    = 18
	ItemSponge       = 19
	ItemGlass        = 20
	ItemLapisOre     = 21
	ItemLapisBlock   = 22
	ItemDispenser    = 23
	ItemSandstone    = 24
	ItemNoteBlock    = 25
	ItemBed          = 26
	ItemPoweredRail  = 27
	ItemDetectorRail = 28
	ItemStickyPiston = 29
	ItemCobweb       = 30
	ItemGrassPlant   = 31
	ItemDeadBush     = 32
	ItemPiston       = 33
	ItemPistonHead   = 34
	ItemWool         = 35
	ItemYellowFlower = 37
	ItemRedFlower    = 38
	ItemBrownMushroom = 39
	ItemRedMushroom  = 40
	ItemGoldBlock    = 41
	ItemIronBlock    = 42
	ItemStoneSlab    = 44
	ItemBricks       = 45
	ItemTNT          = 46
	ItemBookshelf    = 47
	ItemMossStone    = 48
	ItemObsidian     = 49
	ItemTorch        = 50
	ItemFire         = 51
	ItemSpawner      = 52
	ItemOakStairs    = 53
	ItemChest        = 54
	ItemRedstoneWire = 55
	ItemDiamondOre   = 56
	ItemDiamondBlock = 57
	ItemCraftingTable = 58
	ItemWheatCrops   = 59
	ItemFarmland     = 60
	ItemFurnace      = 61
	ItemSign         = 63
	ItemWoodenDoor   = 64
	ItemLadder       = 65
	ItemRails        = 66
	ItemCobblestoneStairs = 67
	ItemWallSign     = 68
	ItemLever        = 69
	ItemStonePlate   = 70
	ItemIronDoor     = 71
	ItemWoodenPlate  = 72
	ItemRedstoneOre  = 73
	ItemRedstoneOreLit = 74
	ItemRedstoneTorch = 76
	ItemStoneButton  = 77
	ItemSnowLayer    = 78
	ItemIce          = 79
	ItemSnowBlock    = 80
	ItemCactus       = 81
	ItemClay         = 82
	ItemSugarCane    = 83
	ItemFence        = 85
	ItemPumpkin      = 86
	ItemNetherrack   = 87
	ItemSoulSand     = 88
	ItemGlowstone    = 89
	ItemPortal       = 90
	ItemJackOLantern = 91
	ItemCake         = 92
	ItemRedstoneRepeater = 93
	ItemRedstoneRepeaterPowered = 94
	ItemWhiteStainedGlass = 95
	ItemTrapdoor     = 96
	ItemStoneBricks  = 98
	ItemIronBars     = 101
	ItemGlassPane    = 102
	ItemMelon        = 103
	ItemPumpkinStem  = 104
	ItemVines        = 106
	ItemFenceGate    = 107
	ItemBrickStairs  = 108
	ItemStoneBrickStairs = 109
	ItemMycelium     = 110
	ItemLilyPad      = 111
	ItemNetherBrick  = 112
	ItemNetherBrickFence = 113
	ItemNetherBrickStairs = 114
	ItemNetherWart   = 115
	ItemEnchantmentTable = 116
	ItemBrewingStand = 117
	ItemCauldron     = 118
	ItemEndPortal    = 119
	ItemEndPortalFrame = 120
	ItemEndStone     = 121
	ItemDragonEgg    = 122
	ItemRedstoneLamp = 123
	ItemRedstoneLampLit = 124
	ItemOakSlab      = 126
	ItemCocoa        = 127
	ItemSandstoneStairs = 128
	ItemEmeraldOre   = 129
	ItemEnderChest   = 130
	ItemTripwireHook = 131
	ItemTripwire     = 132
	ItemEmeraldBlock = 133
	ItemSpruceStairs = 134
	ItemBirchStairs  = 135
	ItemJungleStairs = 136
	ItemCommandBlock = 137
	ItemBeacon       = 138
	ItemCobblestoneWall = 139
	ItemFlowerPot    = 140
	ItemCarrots      = 141
	ItemPotatoes     = 142
	ItemWoodenButton = 143
	ItemHead         = 144
	ItemAnvil        = 145
	ItemTrappedChest = 146
	ItemWeightedPlate = 147
	ItemComparator   = 149
	ItemComparatorPowered = 150
	ItemDaylightDetector = 151
	ItemRedstoneBlock = 152
	ItemQuartzOre    = 153
	ItemHopper       = 154
	ItemQuartzBlock  = 155
	ItemQuartzStairs = 156
	ItemActivatorRail = 157
	ItemDropper      = 158
	ItemWhiteTerracotta = 159
	ItemGlassPaneWhite = 160
	ItemAcaciaLeaves = 161
	ItemAcaciaLog    = 162
	ItemAcaciaStairs = 163
	ItemDarkOakStairs = 164
	ItemSlimeBlock   = 165
	ItemBarrier      = 166
	ItemIronTrapdoor = 167
	ItemPrismarine   = 168
	ItemSeaLantern   = 169
	ItemHayBale      = 170
	ItemCarpet       = 171
	ItemTerracotta   = 172
	ItemCoalBlock    = 173
	ItemPackedIce    = 174
	ItemSunflower    = 175
	ItemBanner       = 176
	ItemWallBanner   = 177
	ItemRedSandstone = 179
	ItemRedSandstoneStairs = 180
	ItemRedSandstoneSlab = 181
	ItemSpruceFenceGate = 183
	ItemBirchFenceGate = 184
	ItemJungleFenceGate = 185
	ItemDarkOakFenceGate = 186
	ItemAcaciaFenceGate = 187
	ItemSpruceFence    = 188
	ItemBirchFence     = 189
	ItemJungleFence    = 190
	ItemDarkOakFence   = 191
	ItemAcaciaFence    = 192
	ItemSpruceDoor     = 193
	ItemBirchDoor      = 194
	ItemJungleDoor     = 195
	ItemDarkOakDoor    = 196
	ItemAcaciaDoor     = 197
	ItemEndRod         = 198
	ItemChorusPlant    = 199
	ItemChorusFlower   = 200
	ItemPurpurBlock    = 201
	ItemPurpurPillar   = 202
	ItemPurpurStairs   = 203
	ItemPurpurSlab     = 205
	ItemEndStoneBricks = 206
	ItemBeetroots      = 207
	ItemGrassPath      = 208
	ItemMagmaBlock     = 213
	ItemNetherWartBlock = 214
	ItemRedNetherBrick = 215
	ItemBoneBlock      = 216
	ItemStructureBlock = 255
	
	// 物品（非方块）
	ItemIronShovel     = 256
	ItemIronPickaxe    = 257
	ItemIronAxe        = 258
	ItemFlintAndSteel  = 259
	ItemApple          = 260
	ItemBow            = 261
	ItemArrow          = 262
	ItemCoal           = 263
	ItemDiamond        = 264
	ItemIronIngot      = 265
	ItemGoldIngot      = 266
	ItemIronSword      = 267
	ItemWoodenSword    = 268
	ItemWoodenShovel   = 269
	ItemWoodenPickaxe  = 270
	ItemWoodenAxe      = 271
	ItemStoneSword     = 272
	ItemStoneShovel    = 273
	ItemStonePickaxe   = 274
	ItemStoneAxe       = 275
	ItemDiamondSword   = 276
	ItemDiamondShovel  = 277
	ItemDiamondPickaxe = 278
	ItemDiamondAxe     = 279
	ItemStick          = 280
	ItemBowl           = 281
	ItemMushroomStew   = 282
	ItemGoldenSword    = 283
	ItemGoldenShovel   = 284
	ItemGoldenPickaxe  = 285
	ItemGoldenAxe      = 286
	ItemString         = 287
	ItemFeather        = 288
	ItemGunpowder      = 289
	ItemWoodenHoe      = 290
	ItemStoneHoe       = 291
	ItemIronHoe        = 292
	ItemDiamondHoe     = 293
	ItemGoldenHoe      = 294
	ItemWheatSeeds     = 295
	ItemWheat          = 296
	ItemBread          = 297
	ItemLeatherHelmet  = 298
	ItemLeatherChestplate = 299
	ItemLeatherLeggings = 300
	ItemLeatherBoots   = 301
	ItemChainmailHelmet = 302
	ItemChainmailChestplate = 303
	ItemChainmailLeggings = 304
	ItemChainmailBoots = 305
	ItemIronHelmet     = 306
	ItemIronChestplate = 307
	ItemIronLeggings   = 308
	ItemIronBoots      = 309
	ItemDiamondHelmet  = 310
	ItemDiamondChestplate = 311
	ItemDiamondLeggings = 312
	ItemDiamondBoots   = 313
	ItemGoldenHelmet   = 314
	ItemGoldenChestplate = 315
	ItemGoldenLeggings = 316
	ItemGoldenBoots    = 317
	ItemFlint          = 318
	ItemPorkchop       = 319
	ItemCookedPorkchop = 320
	ItemPainting       = 321
)

// NewItemStack 创建新的物品堆叠
func NewItemStack(itemID int32, count byte) ItemStack {
	return ItemStack{
		ItemID: itemID,
		Count:  count,
		Damage: 0,
	}
}

