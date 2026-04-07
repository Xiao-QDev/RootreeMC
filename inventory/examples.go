// Package inventory 物品NBT使用示例
package inventory

import "fmt"

// ExampleEnchantedSword 创建附魔钻石剑示例
func ExampleEnchantedSword() ItemStack {
	// 附魔列表
	enchantments := []Enchantment{
		{ID: EnchantmentSharpness, Level: 5}, // 锋利V
		{ID: EnchantmentUnbreaking, Level: 3}, // 耐久III
		{ID: EnchantmentFireAspect, Level: 2}, // 火焰附加II
		{ID: EnchantmentLooting, Level: 3},    // 抢夺III
	}
	
	// 创建附魔钻石剑
	sword := NewEnchantedItem(ItemDiamondSword, 1, enchantments)
	
	fmt.Printf("创建了附魔钻石剑 (ID: %d, NBT大小: %d bytes)\n", 
		sword.ItemID, len(sword.NBTData))
	
	return sword
}

// ExampleUnbreakablePickaxe 创建无法破坏的镐
func ExampleUnbreakablePickaxe() ItemStack {
	// 创建无法破坏的钻石镐
	pickaxe := NewUnbreakableItem(ItemDiamondPickaxe, 1)
	
	// 添加效率V附魔
	enchantments := []Enchantment{
		{ID: EnchantmentEfficiency, Level: 5},
	}
	
	// 重新创建带附魔的无法破坏物品
	pickaxe = NewEnchantedItem(ItemDiamondPickaxe, 1, enchantments)
	
	// 标记为无法破坏（需要手动添加）
	if pickaxe.HasNBT && len(pickaxe.NBTData) > 0 {
		nbt := pickaxe.GetItemNBT()
		nbt.Unbreakable = true
		
		// 重新序列化
		err := pickaxe.serializeNBT(nbt)
		if err != nil {
			fmt.Printf("序列化失败: %v\n", err)
		}
	}
	
	fmt.Printf("创建了无法破坏的钻石镐\n")
	
	return pickaxe
}

// ExampleCustomNamedItem 创建自定义名称的物品
func ExampleCustomNamedItem() ItemStack {
	// 创建一个名为"传说之剑"的钻石剑
	// 使用Minecraft颜色代码：§6=金色，§l=粗体
	sword := NewItemWithCustomName(ItemDiamondSword, 1, "§6§l传说之剑")
	
	fmt.Printf("创建了自定义名称的物品: 传说之剑\n")
	
	return sword
}

// ExampleDamagedItem 创建损坏的物品
func ExampleDamagedItem() ItemStack {
	// 创建一个损坏了500耐久的钻石镐（钻石镐总耐久1561）
	damagedPickaxe := NewDamagedItem(ItemDiamondPickaxe, 1, 500)
	
	fmt.Printf("创建了损坏的钻石镐（耐久度: 500/1561）\n")
	
	return damagedPickaxe
}

// ExampleItemWithLore 创建带描述的物品
func ExampleItemWithLore() ItemStack {
	// 创建基础物品
	item := ItemStack{
		ItemID: ItemDiamond,
		Count:  1,
		Damage: 0,
		HasNBT: true,
	}
	
	// 创建NBT数据
	nbt := &ItemNBT{
		DisplayName: "神秘钻石",
		Lore: []string{
			"§7传说中拥有神秘力量的钻石",
			"§5据说来自远古时代",
			"",
			"§c警告: 请勿滥用！",
		},
	}
	
	// 序列化
	err := item.serializeNBT(nbt)
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
	}
	
	fmt.Printf("创建了带描述的物品: %s\n", nbt.DisplayName)
	fmt.Printf("描述行数: %d\n", len(nbt.Lore))
	
	return item
}

// TestAllNBTItems 测试所有NBT物品类型
func TestAllNBTItems() {
	fmt.Println("=== 测试物品NBT系统 ===\n")
	
	// 1. 附魔武器
	fmt.Println("1. 附魔钻石剑:")
	sword := ExampleEnchantedSword()
	if sword.HasNBT {
		nbt := sword.GetItemNBT()
		fmt.Printf("   附魔数量: %d\n", len(nbt.Enchantments))
		for _, ench := range nbt.Enchantments {
			fmt.Printf("   - %s (等级: %d)\n", ench.ID, ench.Level)
		}
	}
	fmt.Println()
	
	// 2. 无法破坏的工具
	fmt.Println("2. 无法破坏的钻石镐:")
	pickaxe := ExampleUnbreakablePickaxe()
	if pickaxe.HasNBT {
		nbt := pickaxe.GetItemNBT()
		fmt.Printf("   无法破坏: %v\n", nbt.Unbreakable)
		fmt.Printf("   附魔数量: %d\n", len(nbt.Enchantments))
	}
	fmt.Println()
	
	// 3. 自定义名称
	fmt.Println("3. 自定义名称物品:")
	namedItem := ExampleCustomNamedItem()
	if namedItem.HasNBT {
		nbt := namedItem.GetItemNBT()
		fmt.Printf("   名称: %s\n", nbt.DisplayName)
	}
	fmt.Println()
	
	// 4. 损坏的物品
	fmt.Println("4. 损坏的物品:")
	damagedItem := ExampleDamagedItem()
	fmt.Printf("   耐久度: %d\n", damagedItem.Damage)
	fmt.Println()
	
	// 5. 带描述的物品
	fmt.Println("5. 带描述的物品:")
	loreItem := ExampleItemWithLore()
	if loreItem.HasNBT {
		nbt := loreItem.GetItemNBT()
		fmt.Printf("   名称: %s\n", nbt.DisplayName)
		fmt.Printf("   描述:\n")
		for _, line := range nbt.Lore {
			fmt.Printf("     %s\n", line)
		}
	}
	
	fmt.Println("\n=== 测试完成 ===")
}
