package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/framely/sgdnlu/generate_framely/framely"
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"github.com/naturali/CrossWOZ/generate_framely/crosswoz"
	"github.com/naturali/CrossWOZ/generate_framely/dialog"
	"github.com/naturali/CrossWOZ/generate_framely/generate"
)

var (
	mode = flag.String("mode", "expression", "what task to run? "+
		"\t expression: generate expressions for the overall agent\n"+
		"\t agent: generate agent based on the database files\n"+
		"\t aggregate: aggregate dialogues to find out dialogue act combinations and intent/slot combinations\n"+
		"\t full: run the full task including all the above ones")
	dialogueFile = flag.String("dialog-file", "demo10034", "the dialogue json file name")
)

func main() {
	if *mode == "agent" || *mode == "all" {
		agent := generate.GenerateAgent("data/crosswoz/database", "agents")
		framely.OutputAgent(agent, "agents")

		dialog.AllIntents, dialog.AllSlots = VerifyAgent(agent)
	}
	if *mode == "aggregate" || *mode == "all" {
		dialogues := crosswoz.ReadDialogues("data/crosswoz/test.json")
		dialog.AnalyseUserTurns(dialogues, "test", "agents")
	}
	if *mode == "expression" || *mode == "all" {
		inputFile := *dialogueFile
		dialogues := crosswoz.ReadDialogues("data/crosswoz/" + inputFile + ".json")
		expressions := generate.GenerateExpressions(nil, nil, dialogues)
		//for _, exp := range expressions {
		//	framely.ConvertExpressionAnnotationsToDollars(exp, func(annoLabel string) string {
		//		return annoLabel
		//	}, true)
		//}
		b, _ := json.MarshalIndent(expressions, "", "  ")
		outputDir := path.Join("agents", inputFile)
		os.MkdirAll(outputDir, 0755)
		outputFile := path.Join(outputDir, "expression.json")

		if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
			log.Fatal("Failed to write expressions:", err)
		}
		log.Println("Wrote expressions to", outputFile)
		//framely.OutputExpressions(expressions, "agents", inputFile)
	}

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
