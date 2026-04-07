 // Package inventory 物品NBT系统
 package inventory
 
 import (
 	"RootreeMC/nbt"
 	"fmt"
 )
 
 // ItemNBT 物品NBT数据
 type ItemNBT struct {
 	Damage        int16        // 耐久度
 	DisplayName   string       // 显示名称（支持颜色代码）
 	Lore          []string     // 物品描述
 	Enchantments  []Enchantment // 附魔
 	HideFlags     int32        // 隐藏哪些信息
 	Unbreakable   bool         // 是否无法破坏
 }
 
 // Enchantment 附魔
 type Enchantment struct {
 	ID    int16 // 1.12.2 使用数字ID
 	Level int16 // 附魔等级
 }
 
 // NewEnchantedItem 创建附魔物品
 func NewEnchantedItem(itemID int32, count byte, enchantments []Enchantment) ItemStack {
 	item := ItemStack{
 		ItemID: itemID,
 		Count:  count,
 		Damage: 0,
 		HasNBT: true,
 	}
 	
 	// 创建NBT
 	itemNBT := &ItemNBT{
 		Enchantments: enchantments,
 	}
 	
 	// 序列化NBT数据
 	err := item.serializeNBT(itemNBT)
 	if err != nil {
 		fmt.Printf("[ItemNBT] 序列化失败: %v\n", err)
 		item.HasNBT = false
 	}
 	
 	return item
 }
 
 // NewItemWithCustomName 创建带有自定义名称的物品
 func NewItemWithCustomName(itemID int32, count byte, name string) ItemStack {
 	item := ItemStack{
 		ItemID: itemID,
 		Count:  count,
 		Damage: 0,
 		HasNBT: true,
 	}
 	
 	itemNBT := &ItemNBT{
 		DisplayName: name,
 	}
 	
 	err := item.serializeNBT(itemNBT)
 	if err != nil {
 		fmt.Printf("[ItemNBT] 序列化失败: %v\n", err)
 		item.HasNBT = false
 	}
 	
 	return item
 }
 
 // NewUnbreakableItem 创建无法破坏的物品
 func NewUnbreakableItem(itemID int32, count byte) ItemStack {
 	item := ItemStack{
 		ItemID: itemID,
 		Count:  count,
 		Damage: 0,
 		HasNBT: true,
 	}
 	
 	itemNBT := &ItemNBT{
 		Unbreakable: true,
 	}
 	
 	err := item.serializeNBT(itemNBT)
 	if err != nil {
 		fmt.Printf("[ItemNBT] 序列化失败: %v\n", err)
 		item.HasNBT = false
 	}
 	
 	return item
 }
 
 // NewDamagedItem 创建损坏的物品（设置耐久度）
 func NewDamagedItem(itemID int32, count byte, damage int16) ItemStack {
 	item := ItemStack{
 		ItemID: itemID,
 		Count:  count,
 		Damage: damage,
 		HasNBT: true,
 	}
 	
 	itemNBT := &ItemNBT{
 		Damage: damage,
 	}
 	
 	err := item.serializeNBT(itemNBT)
 	if err != nil {
 		fmt.Printf("[ItemNBT] 序列化失败: %v\n", err)
 		item.HasNBT = false
 	}
 	
 	return item
 }
 
 // serializeNBT 序列化NBT数据
 func (item *ItemStack) serializeNBT(itemNBT *ItemNBT) error {
 	tag := nbt.NewCompoundTag()
 	
 	// 耐久度 (1.12.2 通常在 Slot Data 中处理，但 NBT 中也可以存)
 	if itemNBT.Damage != 0 {
 		tag.Set("Damage", &nbt.IntTag{Value: int32(itemNBT.Damage)})
 	}
 	
 	// 显示信息（名称和描述）
 	if itemNBT.DisplayName != "" || len(itemNBT.Lore) > 0 {
 		display := nbt.NewCompoundTag()
 		
 		if itemNBT.DisplayName != "" {
 			display.Set("Name", &nbt.StringTag{Value: itemNBT.DisplayName})
 		}
 		
 		if len(itemNBT.Lore) > 0 {
 			loreList := nbt.NewListTag(nbt.TagString)
 			for _, line := range itemNBT.Lore {
 				loreList.Append(&nbt.StringTag{Value: line})
 			}
 			display.Set("Lore", loreList)
 		}
 		
 		tag.Set("display", display)
 	}
 	
 	// 附魔 (1.12.2 的 Key 是 "ench")
 	if len(itemNBT.Enchantments) > 0 {
 		enchantList := nbt.NewListTag(nbt.TagCompound)
 		
 		for _, ench := range itemNBT.Enchantments {
 			enchantTag := nbt.NewCompoundTag()
 			enchantTag.Set("id", &nbt.ShortTag{Value: ench.ID})
 			enchantTag.Set("lvl", &nbt.ShortTag{Value: ench.Level})
 			enchantList.Append(enchantTag)
 		}
 		
 		tag.Set("ench", enchantList)
 	}
 	
 	// 隐藏标志
 	if itemNBT.HideFlags != 0 {
 		tag.Set("HideFlags", &nbt.IntTag{Value: itemNBT.HideFlags})
 	}
 	
 	// 无法破坏
 	if itemNBT.Unbreakable {
 		tag.Set("Unbreakable", &nbt.ByteTag{Value: 1})
 	}
 	
 	// 序列化NBT (使用 WriteAnonymousBytes 确保没有根标签名称)
 	nbtData := &nbt.NBT{
 		Name: "",
 		Root: tag,
 	}
 	
 	data, err := nbtData.WriteAnonymousBytes()
 	if err != nil {
 		return fmt.Errorf("NBT序列化失败: %v", err)
 	}
 	
 	item.NBTData = data
 	return nil
 }
 
 // deserializeNBT 反序列化NBT数据
 func (item *ItemStack) deserializeNBT() (*ItemNBT, error) {
 	if !item.HasNBT || len(item.NBTData) == 0 {
 		return &ItemNBT{}, nil
 	}
 	
 	// 使用 ReadAnonymousBytes
 	nbtData, err := nbt.ReadAnonymousBytes(item.NBTData)
 	if err != nil {
 		return nil, fmt.Errorf("NBT反序列化失败: %v", err)
 	}
 	
 	compound, ok := nbtData.Root.(*nbt.CompoundTag)
 	if !ok {
 		return nil, fmt.Errorf("NBT根标签不是复合标签")
 	}
 	
 	itemNBT := &ItemNBT{}
 	
 	// 读取耐久度
 	if damage, ok := compound.GetInt("Damage"); ok {
 		itemNBT.Damage = int16(damage)
 	}
 	
 	// 读取显示信息
 	if display, ok := compound.GetCompound("display"); ok {
 		if name, ok := display.GetString("Name"); ok {
 			itemNBT.DisplayName = name
 		}
 		
 		// 读取描述
 		if loreList, ok := display.GetList("Lore"); ok {
 			for _, tag := range loreList.Value {
 				if strTag, ok := tag.(*nbt.StringTag); ok {
 					itemNBT.Lore = append(itemNBT.Lore, strTag.Value)
 				}
 			}
 		}
 	}
 	
 	// 读取附魔 (1.12.2 尝试读取 "ench")
 	enchList, ok := compound.GetList("ench")
 	if !ok {
 		// 兼容性检查
 		enchList, ok = compound.GetList("Enchantments")
 	}
 	
 	if ok {
 		for _, tag := range enchList.Value {
 			if enchTag, ok := tag.(*nbt.CompoundTag); ok {
 				enchantment := Enchantment{}
 				
 				if id, ok := enchTag.GetShort("id"); ok {
 					enchantment.ID = id
 				}
 				
 				if lvl, ok := enchTag.GetShort("lvl"); ok {
 					enchantment.Level = lvl
 				}
 				
 				itemNBT.Enchantments = append(itemNBT.Enchantments, enchantment)
 			}
 		}
 	}
 	
 	// 读取隐藏标志
 	if hideFlags, ok := compound.GetInt("HideFlags"); ok {
 		itemNBT.HideFlags = hideFlags
 	}
 	
 	// 读取无法破坏
 	if unbreakable, ok := compound.GetByte("Unbreakable"); ok {
 		itemNBT.Unbreakable = (unbreakable == 1)
 	}
 	
 	return itemNBT, nil
 }
 
 // GetItemNBT 获取物品的NBT数据（如果不存在则反序列化）
 func (item *ItemStack) GetItemNBT() *ItemNBT {
 	if !item.HasNBT || len(item.NBTData) == 0 {
 		return &ItemNBT{}
 	}
 	
 	itemNBT, err := item.deserializeNBT()
 	if err != nil {
 		fmt.Printf("[ItemNBT] 反序列化失败: %v\n", err)
 		return &ItemNBT{}
 	}
 	
 	return itemNBT
 }
 
 // 附魔ID常量 (1.12.2 数字 ID)
 const (
 	EnchantmentProtection       = 0
 	EnchantmentFireProtection   = 1
 	EnchantmentFeatherFalling   = 2
 	EnchantmentBlastProtection  = 3
 	EnchantmentProjectileProtection = 4
 	EnchantmentRespiration      = 5
 	EnchantmentAquaAffinity     = 6
 	EnchantmentThorns           = 7
 	EnchantmentDepthStrider     = 8
 	EnchantmentFrostWalker      = 9
 	EnchantmentBindingCurse     = 10
 	EnchantmentSharpness        = 16
 	EnchantmentSmite            = 17
 	EnchantmentBaneOfArthropods = 18
 	EnchantmentKnockback        = 19
 	EnchantmentFireAspect       = 20
 	EnchantmentLooting          = 21
 	EnchantmentSweepingEdge     = 22
 	EnchantmentEfficiency       = 32
 	EnchantmentSilkTouch        = 33
 	EnchantmentUnbreaking       = 34
 	EnchantmentFortune          = 35
 	EnchantmentPower            = 48
 	EnchantmentPunch            = 49
 	EnchantmentFlame            = 50
 	EnchantmentInfinity         = 51
 	EnchantmentLuckOfTheSea     = 61
 	EnchantmentLure             = 62
 	EnchantmentMending          = 70
 	EnchantmentVanishingCurse   = 71
 )
