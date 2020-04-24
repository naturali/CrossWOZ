package dialog

import (
	"encoding/json"
	"github.com/framely/sgdnlu/generate_framely/sgd"
	"github.com/krystollia/CrossWOZ/generate_framely/crosswoz"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strings"
)

type Cnt struct {
	Turns     int
	Dialogues int
}

func AnalyseUserTurnsREQUEST(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	var userTurnRequestSlots = make(map[string]*Cnt)
	var userTurnRequestIntents = make(map[string]*Cnt)
	for _, dialog := range dialogues {
		slotsMap := make(map[string]bool)
		intentsMap := make(map[string]bool)
		log.Println("analyse request ~~~", dialog.DialogueID)
		for _, turn := range dialog.Turns {
			if turn.Speaker != "usr" {
				continue
			}
			var requestSlots []string
			var requestIntents []string
			for _, act := range turn.DialogActs {
				if act.Act == "Request" {
					slotName := act.Intent + "." + act.Slot
					if act.Value != "" {
						log.Println("!!!!!!!!!!!!!!!", act, turn.Utterance)
						slotName = slotName + "." + act.Value
					}
					requestSlots = sgd.AppendIfNotExists(requestSlots, slotName)
					requestIntents = sgd.AppendIfNotExists(requestIntents, act.Intent)
				}
			}
			if len(requestSlots) == 0 {
				continue
			}
			// check requested slots combinations
			sort.Strings(requestSlots)
			slotsKey := strings.Join(requestSlots, ",")
			if _, ok := userTurnRequestSlots[slotsKey]; !ok {
				userTurnRequestSlots[slotsKey] = &Cnt{
					Turns:     1,
					Dialogues: 1,
				}
			} else {
				userTurnRequestSlots[slotsKey].Turns++
				if _, ok := slotsMap[slotsKey]; !ok {
					userTurnRequestSlots[slotsKey].Dialogues++
				}
			}
			slotsMap[slotsKey] = true
			// check requested intent combinations
			sort.Strings(requestIntents)
			intentsKey := strings.Join(requestIntents, ",")
			if _, ok := userTurnRequestIntents[intentsKey]; !ok {
				userTurnRequestIntents[intentsKey] = &Cnt{
					Turns:     1,
					Dialogues: 1,
				}
			} else {
				userTurnRequestIntents[intentsKey].Turns++
				if _, ok := intentsMap[intentsKey]; !ok {
					userTurnRequestIntents[intentsKey].Dialogues++
				}
			}
			intentsMap[intentsKey] = true
		}
	}
	os.MkdirAll(outputDir, 0755)
	b, _ := json.MarshalIndent(userTurnRequestSlots, "", "  ")
	outputFile := path.Join(outputDir, inputFile+"_userRequestedSlots.json")
	if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
		log.Fatal("Failed to write file ", outputFile, err)
	}
	log.Println("Wrote user request slots to", outputFile)

	b, _ = json.MarshalIndent(userTurnRequestIntents, "", "  ")
	outputFile = path.Join(outputDir, inputFile+"_userRequestedIntentss.json")
	if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
		log.Fatal("Failed to write file ", outputFile, err)
	}
	log.Println("Wrote user request intents to", outputFile)
}

func AnalyseUserTurns(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	var userTurnActs = make(map[string]*Cnt)
	for _, dialog := range dialogues {
		log.Println("~~~", dialog.DialogueID)
		actCombinations := make(map[string]bool)
		for _, turn := range dialog.Turns {
			if turn.Speaker != "usr" {
				continue
			}
			actMap := make(map[string]bool)
			for _, act := range turn.DialogActs {
				actMap[act.Act] = true
			}
			var acts []string
			for act := range actMap {
				acts = append(acts, act)
			}
			sort.Strings(acts)
			combination := strings.Join(acts, ",")
			log.Println("---", combination)
			if _, ok := userTurnActs[combination]; !ok {
				userTurnActs[combination] = &Cnt{Turns: 1, Dialogues: 1}
			} else {
				userTurnActs[combination].Turns++
				if _, ok := actCombinations[combination]; !ok {
					log.Println("---!!!", combination)
					userTurnActs[combination].Dialogues++
				}
			}
			actCombinations[combination] = true
		}
	}
	os.MkdirAll(outputDir, 0755)
	b, _ := json.MarshalIndent(userTurnActs, "", "  ")
	outputFile := path.Join(outputDir, inputFile+"_userActCombinations.json")
	if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
		log.Fatal("Failed to write file ", outputFile, err)
	}
	log.Println("Wrote user act combinations to", outputFile)
}
