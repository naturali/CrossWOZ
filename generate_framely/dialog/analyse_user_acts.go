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
	requestSlotsExtractor := func(turn *crosswoz.Message) string {
		var requestSlots []string
		for _, act := range turn.DialogActs {
			if act.Act == "Request" {
				slotName := act.Intent + "." + act.Slot
				if act.Value != "" {
					log.Println("request with a value???????? ", act, turn.Utterance)
					slotName = slotName + "." + act.Value
				}
				requestSlots = sgd.AppendIfNotExists(requestSlots, slotName)
			}
		}
		sort.Strings(requestSlots)
		return strings.Join(requestSlots, ",")
	}

	requestIntentsExtractor := func(turn *crosswoz.Message) string {
		var requestIntents []string
		for _, act := range turn.DialogActs {
			if act.Act == "Request" {
				requestIntents = sgd.AppendIfNotExists(requestIntents, act.Intent)
			}
		}
		sort.Strings(requestIntents)
		return strings.Join(requestIntents, ",")
	}

	AnalyseUserTurns(dialogues, inputFile, outputDir, "userRequestedSlots", requestSlotsExtractor, true)
	AnalyseUserTurns(dialogues, inputFile, outputDir, "userRequestedIntentss", requestIntentsExtractor, true)
}

func AnalyseUserTurnActCombinations(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AnalyseUserTurns(dialogues, inputFile, outputDir, "userActCombinations", func(turn *crosswoz.Message) string {
		actMap := make(map[string]bool)
		for _, act := range turn.DialogActs {
			actMap[act.Act] = true
		}
		var acts []string
		for act := range actMap {
			acts = append(acts, act)
		}
		sort.Strings(acts)
		return strings.Join(acts, ",")
	}, false)
}

func AnalyseUserTurns(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string, subject string, subjectExtractor func(turn *crosswoz.Message) string, ignoreEmptySubject bool) {
	var aggregationForAllDialogues = make(map[string]*Cnt)
	for _, dialog := range dialogues {
		log.Println("----- analysing", dialog.DialogueID, "for subject:", subject)
		seen := make(map[string]bool)
		for _, turn := range dialog.Turns {
			if turn.Speaker != "usr" {
				continue
			}
			subjectValue := subjectExtractor(turn)
			if ignoreEmptySubject && subjectValue == "" {
				continue
			}
			log.Println(subjectValue)
			if _, ok := aggregationForAllDialogues[subjectValue]; !ok {
				aggregationForAllDialogues[subjectValue] = &Cnt{Turns: 1, Dialogues: 1}
			} else {
				aggregationForAllDialogues[subjectValue].Turns++
				if _, ok := seen[subjectValue]; !ok {
					log.Println("new seen subject:", subjectValue)
					aggregationForAllDialogues[subjectValue].Dialogues++
				}
			}
			seen[subjectValue] = true
		}
	}
	os.MkdirAll(outputDir, 0755)
	b, _ := json.MarshalIndent(aggregationForAllDialogues, "", "  ")
	outputFile := path.Join(outputDir, inputFile+"_"+subject+".json")
	if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
		log.Fatal("Failed to write file ", outputFile, err)
	}
	log.Println("Wrote "+subject+" aggregation to", outputFile)
}
