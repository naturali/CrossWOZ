package main

import (
	"github.com/framely/sgdnlu/generate_framely/framely"
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"github.com/naturali/CrossWOZ/generate_framely/crosswoz"
	"github.com/naturali/CrossWOZ/generate_framely/dialog"
	"github.com/naturali/CrossWOZ/generate_framely/generate"
	"log"
)

func main() {
	agent := generate.GenerateAgent("data/crosswoz/database", "agents")
	framely.OutputAgent(agent, "agents")

	dialog.AllIntents, dialog.AllSlots = VerifyAgent(agent)
	dialogues := crosswoz.ReadDialogues("data/crosswoz/test.json")
	dialog.AnalyseUserTurns(dialogues, "test", "agents")
}

func VerifyAgent(agent *p.Agent) (allIntentIDs map[string]bool, allSlotIDs map[string]bool) {
	allTypeIDs := make(map[string]bool)
	// check entities, should not have duplicated type ids
	for _, ent := range agent.Entities {
		if _, ok := allTypeIDs[ent.TypeId]; ok {
			log.Fatal("Duplicate entity type: ", ent.TypeId)
		}
		allTypeIDs[ent.TypeId] = true
	}
	allIntentIDs = make(map[string]bool)
	allSlotIDs = make(map[string]bool)
	// check slots, all slots' type id should exist
	for _, intent := range agent.Intents {
		allIntentIDs[intent.MetaId] = true
		for _, slot := range intent.Slots {
			if _, ok := allTypeIDs[slot.TypeId]; !ok {
				log.Fatal("Unknown slot type id: ", slot.TypeId)
			}
			allSlotIDs[slot.AttributeId] = true
		}
	}
	log.Println(allTypeIDs)
	log.Println(allIntentIDs)
	log.Println(allSlotIDs)
	return allIntentIDs, allSlotIDs
}
