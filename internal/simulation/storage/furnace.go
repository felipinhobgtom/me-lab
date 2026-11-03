package storage

import "github.com/felipinho/me-lab/internal/domain"

// Vanilla Furnace/Blast Furnace behavior targets Minecraft Java 1.21.1.
// Sources: https://minecraft.wiki/w/Furnace#Processing_items,
// https://minecraft.wiki/w/Blast_Furnace#Smelting, and
// https://minecraft.wiki/w/Smelting#Fuel.
//
// This explicit table is intentionally limited to the recipes implemented by
// this simulator; inputs absent from it are never processed silently.
var vanillaSmeltingRecipes = map[string]string{
	"minecraft:raw_iron":    "minecraft:iron_ingot",
	"minecraft:raw_gold":    "minecraft:gold_ingot",
	"minecraft:raw_copper":  "minecraft:copper_ingot",
	"minecraft:cobblestone": "minecraft:stone",
	"minecraft:sand":        "minecraft:glass",
	"minecraft:clay_ball":   "minecraft:brick",
}

var blastFurnaceInputs = map[string]bool{
	"minecraft:raw_iron":   true,
	"minecraft:raw_gold":   true,
	"minecraft:raw_copper": true,
}

var vanillaFuelBurnTimes = map[string]int{
	"minecraft:coal":             1600,
	"minecraft:charcoal":         1600,
	"minecraft:blaze_rod":        2400,
	"minecraft:dried_kelp_block": 4000,
	"minecraft:coal_block":       16000,
	"minecraft:lava_bucket":      20000,
}

func fuelBurnTime(itemID string) int {
	return vanillaFuelBurnTimes[itemID]
}

func furnaceInputAllowed(blockID, itemID string) bool {
	if blockID == "minecraft:blast_furnace" && !blastFurnaceInputs[itemID] {
		return false
	}
	_, ok := vanillaSmeltingRecipes[itemID]
	return ok
}

// StepExternalMachines advances every vanilla furnace and blast furnace by one tick.
func (s *State) StepExternalMachines() {
	if s == nil {
		return
	}
	for _, inv := range s.ExternalInventories {
		if inv == nil || !isFurnace(inv.Block.BlockDefID) {
			continue
		}
		stepVanillaFurnace(inv)
	}
}

func stepVanillaFurnace(inv *ExternalInventoryState) {
	blast := inv.Block.BlockDefID == "minecraft:blast_furnace"
	if inv.CookTimeTotal <= 0 {
		inv.CookTimeTotal = furnaceCookTimeTotal(inv.Block.BlockDefID)
	}
	cookTotal := inv.CookTimeTotal
	rate := 1
	if blast {
		rate = 2
	}

	recipeOutput, canSmelt := furnaceRecipeOutput(inv)
	if inv.BurnTime <= 0 {
		inv.Lit = false
		if canSmelt {
			if fuelTime, ok := furnaceFuelTime(inv); ok {
				consumeSlotItem(inv.Fuel)
				inv.BurnTime = fuelTime
				inv.BurnTimeTotal = fuelTime
				inv.Lit = true
			}
		}
	}

	if inv.BurnTime <= 0 {
		inv.Lit = false
		return
	}

	if canSmelt {
		inv.CookTime++
		if inv.CookTime >= cookTotal {
			consumeSlotItem(inv.Input)
			if len(inv.Output) == 0 {
				inv.Output = []domain.ItemStack{recipeOutput}
			} else {
				addSlotItem(inv.Output, recipeOutput)
			}
			inv.CookTime = 0
		}
	} else if len(inv.Input) == 0 || len(inv.Input) == 1 && vanillaSmeltingRecipes[inv.Input[0].ItemID] == "" {
		// An unsupported/empty input has no partial progress to retain.
		inv.CookTime = 0
	}

	inv.BurnTime -= rate
	if inv.BurnTime <= 0 {
		inv.BurnTime = 0
		inv.Lit = false
	}
}

func furnaceRecipeOutput(inv *ExternalInventoryState) (domain.ItemStack, bool) {
	if inv == nil || len(inv.Input) == 0 || inv.Input[0].Quantity <= 0 {
		return domain.ItemStack{}, false
	}
	inputID := inv.Input[0].ItemID
	if inv.Block.BlockDefID == "minecraft:blast_furnace" && !blastFurnaceInputs[inputID] {
		return domain.ItemStack{}, false
	}
	outputID, ok := vanillaSmeltingRecipes[inputID]
	if !ok || !canAddSlotItem(inv.Output, outputID, 1) {
		return domain.ItemStack{}, false
	}
	return domain.ItemStack{ItemID: outputID, Quantity: 1}, true
}

func furnaceFuelTime(inv *ExternalInventoryState) (int, bool) {
	if inv == nil || len(inv.Fuel) == 0 || inv.Fuel[0].Quantity <= 0 {
		return 0, false
	}
	time, ok := vanillaFuelBurnTimes[inv.Fuel[0].ItemID]
	return time, ok
}

func consumeSlotItem(slot []domain.ItemStack) {
	if len(slot) == 0 {
		return
	}
	slot[0].Quantity--
	if slot[0].Quantity <= 0 {
		slot[0].ItemID = ""
		slot[0].Quantity = 0
	}
}

func canAddSlotItem(slot []domain.ItemStack, itemID string, quantity int) bool {
	if itemID == "" || quantity <= 0 {
		return false
	}
	if len(slot) == 0 || slot[0].Quantity <= 0 {
		return true
	}
	return slot[0].ItemID == itemID && slot[0].Quantity <= 64-quantity
}

func addSlotItem(slot []domain.ItemStack, item domain.ItemStack) {
	if item.ItemID == "" || item.Quantity <= 0 {
		return
	}
	if len(slot) == 0 {
		return
	}
	slot[0].ItemID = item.ItemID
	slot[0].Quantity += item.Quantity
}
