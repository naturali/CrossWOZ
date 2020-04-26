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
	Turns      int
	Dialogues  int
	Utterances []string `json:"-"`
}

var (
	AllSlots   map[string]bool
	AllIntents map[string]bool
)

func AnalyseUserTurnActCombinations(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AggregateUserTurns(dialogues, inputFile, outputDir, "userActCombinations", func(turn *crosswoz.Message, turnIdx int) string {
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

func AnalyseUserTurnsREQUEST(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AggregateUserTurns(dialogues, inputFile, outputDir, "userRequestedSlots", slotExtractorForSpecificAct("Request"), true)
	AggregateUserTurns(dialogues, inputFile, outputDir, "userRequestedIntentss", intentExtractorForSpecificAct("Request"), true)
}

func AnalyseUserTurnsSELECT(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AggregateUserTurns(dialogues, inputFile, outputDir, "userSelectedSlots", slotExtractorForSpecificAct("Select"), true)
	AggregateUserTurns(dialogues, inputFile, outputDir, "useSelectedIntents", intentExtractorForSpecificAct("Select"), true)
}

func AnalyseUserTurnsINFORM(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AggregateUserTurns(dialogues, inputFile, outputDir, "userInformedSlots", slotExtractorForSpecificAct("Inform"), true)
	AggregateUserTurns(dialogues, inputFile, outputDir, "userInformedIntents", intentExtractorForSpecificAct("Inform"), true)
}

func slotExtractorForSpecificAct(actType string) func(turn *crosswoz.Message, turnIdx int) string {
	return func(turn *crosswoz.Message, turnIdx int) string {
		var requestSlots []string
		for _, act := range turn.DialogActs {
			if act.Act == actType {
				slotName := act.Intent + "." + act.Slot
				if actType == "Select" {
					log.Printf("Select %s = %s, %s %d", slotName, act.Value, turn.Utterance, turnIdx)
				}
				if AllSlots != nil && !AllSlots[slotName] && !strings.HasSuffix(slotName, "源领域") {
					log.Fatal("Unknown slot:", slotName, turn.Utterance)
				}
				requestSlots = sgd.AppendIfNotExists(requestSlots, slotName)
			}
		}
		sort.Strings(requestSlots)
		return strings.Join(requestSlots, ",")
	}
}

func intentExtractorForSpecificAct(actType string) func(turn *crosswoz.Message, turnIdx int) string {
	return func(turn *crosswoz.Message, turnIdx int) string {
		var requestIntents []string
		for _, act := range turn.DialogActs {
			if act.Act == actType {
				if AllIntents != nil && !AllIntents[act.Intent] {
					log.Fatal("Unknown intent:", act.Intent, turn.Utterance)
				}
				requestIntents = sgd.AppendIfNotExists(requestIntents, act.Intent)
			}
		}
		sort.Strings(requestIntents)
		return strings.Join(requestIntents, ",")
	}
}

func AggregateUserTurns(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string, subject string, subjectExtractor func(turn *crosswoz.Message, turnIdx int) string, ignoreEmptySubject bool) {
	var aggregationForAllDialogues = make(map[string]*Cnt)
	for _, dialog := range dialogues {
		log.Println("----- analysing", dialog.DialogueID, "for subject:", subject)
		seen := make(map[string]bool)
		for i, turn := range dialog.Turns {
			if turn.Speaker != "usr" {
				continue
			}
			subjectValue := subjectExtractor(turn, i)
			if ignoreEmptySubject && subjectValue == "" {
				continue
			}
			//log.Println(subjectValue)
			if _, ok := aggregationForAllDialogues[subjectValue]; !ok {
				aggregationForAllDialogues[subjectValue] = &Cnt{
					Turns:      1,
					Dialogues:  1,
					Utterances: []string{turn.Utterance},
				}
			} else {
				aggregationForAllDialogues[subjectValue].Turns++
				aggregationForAllDialogues[subjectValue].Utterances = sgd.AppendIfNotExists(aggregationForAllDialogues[subjectValue].Utterances, turn.Utterance)
				if _, ok := seen[subjectValue]; !ok {
					log.Println("new seen", subject, ":", subjectValue)
					aggregationForAllDialogues[subjectValue].Dialogues++
				}
			}
			seen[subjectValue] = true
		}
	}
	os.MkdirAll(path.Join(outputDir, inputFile, "dialogue_aggregate"), 0755)
	b, _ := json.MarshalIndent(aggregationForAllDialogues, "", "  ")
	outputFile := path.Join(outputDir, inputFile, "dialogue_aggregate", subject+".json")
	if err := ioutil.WriteFile(outputFile, b, 0755); err != nil {
		log.Fatal("Failed to write file ", outputFile, err)
	}
	log.Println("Wrote "+subject+" aggregation to", outputFile)
}

func AnalyseUserTurns(dialogues []*crosswoz.Dialogue, inputFile string, outputDir string) {
	AnalyseUserTurnActCombinations(dialogues, inputFile, outputDir)
	AnalyseUserTurnsREQUEST(dialogues, inputFile, outputDir)
	AnalyseUserTurnsSELECT(dialogues, inputFile, outputDir)
	AnalyseUserTurnsINFORM(dialogues, inputFile, outputDir)
}
