package main

import (
	"encoding/json"
	"github.com/framely/sgdnlu/generate_framely/framely"
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"github.com/framely/sgdnlu/generate_framely/sgd"
	"github.com/krystollia/CrossWOZ/generate_framely/crosswoz"
	"github.com/krystollia/CrossWOZ/generate_framely/dialog"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type SlotIdentifier struct {
	FullName string
	IsMulti  bool
}

type SlotGroup struct {
	ID           int
	GroupName    string
	SlotNames    map[string]*SlotIdentifier
	SrcDialogues map[string]bool
}

// if slot names are the same, merge different slot groups by group name
type MergedSlotGroup struct {
	IDs          []int
	GroupName    string
	Slots        []*SlotIdentifier
	SrcDialogues []string
}

func MapKeysSorted(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func NewMergedSlotGroup(group *SlotGroup) *MergedSlotGroup {
	mergedGroup := MergedSlotGroup{
		IDs:          []int{group.ID},
		GroupName:    group.GroupName,
		SrcDialogues: MapKeysSorted(group.SrcDialogues),
	}
	for _, slot := range group.SlotNames {
		mergedGroup.Slots = append(mergedGroup.Slots, slot)
	}
	sort.Slice(mergedGroup.Slots, func(i, j int) bool {
		return mergedGroup.Slots[i].FullName < mergedGroup.Slots[j].FullName
	})
	return &mergedGroup
}

func CompareMergedSlotGroupsBySlots(group1, group2 *MergedSlotGroup) bool {
	g1 := &MergedSlotGroup{
		Slots: group1.Slots,
	}
	g2 := &MergedSlotGroup{
		Slots: group2.Slots,
	}

	b1, _ := json.Marshal(g1)
	b2, _ := json.Marshal(g2)
	same := true
	if string(b1) != string(b2) {
		log.Printf("%s\n%s", string(b1), string(b2))
		same = false
	}
	return same
}

func groupSlots(slotGroups map[string]*SlotGroup, inputDir string, inputFileName string) (groups map[string][]*MergedSlotGroup) {
	groups = make(map[string][]*MergedSlotGroup)
	for _, rawGroup := range slotGroups {
		group := NewMergedSlotGroup(rawGroup)
		if _, ok := groups[rawGroup.GroupName]; !ok {
			groups[rawGroup.GroupName] = []*MergedSlotGroup{group}
		} else {
			needAppend := true
			for _, savedGroup := range groups[rawGroup.GroupName] {
				if CompareMergedSlotGroupsBySlots(group, savedGroup) {
					savedGroup.IDs = append(savedGroup.IDs, group.IDs...)
					sort.Ints(savedGroup.IDs)
					savedGroup.SrcDialogues = append(savedGroup.SrcDialogues, group.SrcDialogues...)
					sort.Strings(savedGroup.SrcDialogues)
					needAppend = false
				}
			}
			if needAppend {
				groups[rawGroup.GroupName] = append(groups[rawGroup.GroupName], group)
				log.Printf("same name, new appended group: %v", *group)
			}

		}
	}
	var groupList = make([]struct {
		Name   string
		Groups []*MergedSlotGroup
	}, len(groups))
	i := 0
	for k := range groups {
		sort.Slice(groups[k], func(i, j int) bool {
			iIDs, _ := json.Marshal(groups[k][i].IDs)
			jIDS, _ := json.Marshal(groups[k][j].IDs)
			return string(iIDs) < string(jIDS)
		})
		groupList[i].Name = k
		groupList[i].Groups = groups[k]
		i++
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].Name < groupList[j].Name
	})
	fileName := path.Join(inputDir, "processed_slots_grouped_"+inputFileName+".json")
	b, err := json.MarshalIndent(groupList, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal slots, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write slots, err:", err)
	} else {
		log.Println("wrote slots to", fileName)
	}

	fileName = path.Join(inputDir, "processed_slots_"+inputFileName+".json")
	b, err = json.MarshalIndent(slotGroups, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal slots, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write slots, err:", err)
	} else {
		log.Println("wrote slots to", fileName)
	}
	return groups
}

// 把每个domain当成一个skill
func GenerateIntents(groups map[string][]*MergedSlotGroup, inputFileName string, outputDir string) {
	var intents []*p.IntentMeta
	for _, group := range groups {
		intent := &p.IntentMeta{
			MetaId:          "CrossWOZ." + group[0].GroupName,
			Name:            group[0].GroupName,
			Responses:       nil, // TODO
			Type:            "intent",
			DataQueries:     nil,
			NewSuggestHooks: nil,
			ContextIntents:  nil,
			Src:             "",
			ParentClassIds:  nil,
		}
		slotMap := make(map[string]bool)
		for _, g := range group {
			if g.GroupName != group[0].GroupName {
				log.Fatal("impossible", g.GroupName, group[0].GroupName)
			}
			for _, slot := range g.Slots {
				if multi, ok := slotMap[slot.FullName]; !ok {
					slotMap[slot.FullName] = slot.IsMulti
				} else {
					if slot.IsMulti != multi {
						log.Fatal("multi? single?", g.GroupName, slot.FullName)
					}
				}
			}
		}
		for slotName, multi := range slotMap {
			intent.Slots = append(intent.Slots, &p.FramelySlot{
				Name:              slotName,
				AttributeId:       slotName,
				TypeId:            "",    // TODO
				AllowAskSlot:      true,  // TODO
				AskSlotPrompt:     nil,   // TODO
				AllowConfirm:      false, // TODO we should get this from dialogue
				ConfirmPrompts:    nil,
				AllowMultiValue:   multi,
				MultiValuePrompts: nil,
				AllowUnknown:      false, // TODO
				AllowSubtype:      false, // TODO
			})
		}
		sort.Slice(intent.Slots, func(i, j int) bool {
			return intent.Slots[i].Name < intent.Slots[j].Name
		})
		intents = append(intents, intent)
	}
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].Name < intents[j].Name
	})
	agent := p.Agent{
		Agent: &p.DHLAgentMeta{
			AgentId:     "CrossWOZ",
			Name:        "CrossWOZ",
			AgentOrg:    "CrossWOZ",
			Description: "generated from CrossWOZ data set",
		},
		Entities:   []*p.BasicTypeMeta{{}},
		Composites: []*p.IntentMeta{{}},
		Intents:    intents,
	}
	os.MkdirAll(path.Join(outputDir, inputFileName), 0755)
	fileName := path.Join(outputDir, inputFileName, "agent.json")
	b, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal agent, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write agent, err:", err)
	} else {
		log.Println("wrote agent to", fileName)
	}
}

func listDomainCombinations(rawDialogues map[string]*crosswoz.RawDialogue, inputFile string) {
	type SlotInfo struct {
		Domain   string
		SlotName string
		Multi    bool
	}

	type Goal struct {
		RequiredSlots []*SlotInfo `json:"-"`
		Dialogues     []string
	}

	var goalKinds = make(map[string]*Goal)
	for dialogID, rawDialogue := range rawDialogues {
		var requiredSlots []*SlotInfo
		for i, rawSlot := range rawDialogue.Goal {
			slot := crosswoz.ParseSlot(rawSlot, dialogID, i)
			requiredSlots = append(requiredSlots, &SlotInfo{
				Domain:   slot.Group,
				SlotName: slot.Name,
				Multi:    slot.IsMulti(dialogID, i),
			})
		}
		sort.Slice(requiredSlots, func(i, j int) bool {
			if requiredSlots[i].Domain == requiredSlots[j].Domain {
				return requiredSlots[i].SlotName < requiredSlots[j].SlotName
			}
			return requiredSlots[i].Domain < requiredSlots[j].Domain
		})
		b, _ := json.Marshal(requiredSlots)
		s := string(b)

		domains := make(map[string]bool)
		for _, slot := range requiredSlots {
			domains[slot.Domain] = true
		}
		var domainList []string
		for domain := range domains {
			domainList = append(domainList, domain)
		}
		sort.Strings(domainList)
		s = strings.Join(domainList, ",")

		if _, ok := goalKinds[s]; !ok {
			goalKinds[s] = &Goal{
				RequiredSlots: requiredSlots,
				Dialogues:     []string{dialogID},
			}
		} else {
			goalKinds[s].Dialogues = append(goalKinds[s].Dialogues, dialogID)
		}
	}

	var domainCombinations = make([]struct {
		Domains   string `json:"domains"`
		Dialogues string `json:"-"`
		DialogCnt int    `json:"dialogs"`
	}, len(goalKinds))
	i := 0
	sum := 0
	for s, goal := range goalKinds {
		domainCombinations[i].Domains = s
		domainCombinations[i].Dialogues = strings.Join(goal.Dialogues, ",")
		domainCombinations[i].DialogCnt = len(goal.Dialogues)
		sum += len(goal.Dialogues)
		i++
	}
	log.Println(sum)
	sort.Slice(domainCombinations, func(i, j int) bool {
		if domainCombinations[i].DialogCnt == domainCombinations[j].DialogCnt {
			return domainCombinations[i].Domains < domainCombinations[j].Domains
		}
		return domainCombinations[i].DialogCnt > domainCombinations[j].DialogCnt
	})
	b, _ := json.MarshalIndent(domainCombinations, "", "  ")
	ioutil.WriteFile("generate_framely/demo/domain_combinations_"+inputFile+".json", b, 0666)
}

func ExtractExpressions(dialogues []*crosswoz.Dialogue, outputDir string, inputFile string) {
	var expressions []*p.FramelyExpression
	for _, dialog := range dialogues {
		for _, msg := range dialog.Turns {
			relatedIntents := msg.RelatedIntents()
			if len(relatedIntents) > 1 {
				log.Fatal("一次触发两个intent？", relatedIntents, msg.Utterance, dialog.DialogueID)
			}
			if len(relatedIntents) == 0 {
				log.Println("没有触发 intent？", relatedIntents, msg.Utterance, dialog.DialogueID)
				continue
			}
			exp := &p.FramelyExpression{
				Utterance: msg.Utterance,
				OwnerId:   relatedIntents[0],
			}
			expressions = append(expressions, exp)
			relatedSlots := msg.RelatedSlots()
			for slotName, slotValue := range relatedSlots.InformedSlots {
				anno := sgd.ExtractAnnotation(msg.Utterance, slotName, []string{slotValue}, dialog.DialogueID)
				if anno != nil {
					exp.Annotations = append(exp.Annotations, anno)
				}
			}
		}
	}
	os.MkdirAll(path.Join(outputDir, inputFile), 0755)
	framely.OutputExpressions(expressions, outputDir, inputFile)

}

func AnalyseGoals(dialogs []*crosswoz.Dialogue, inputDir string, inputFile string) map[string][]*MergedSlotGroup {
	slotGroups := make(map[string]*SlotGroup)
	for _, rawDialogue := range dialogs {
		dialogID := rawDialogue.DialogueID
		slotsGoal := make(map[string]bool)
		for i, rawSlot := range rawDialogue.Goal {
			slot := crosswoz.ParseSlot(rawSlot, dialogID, i)
			if slot.Filled {
				log.Printf("!slot in goal should not be filled, dialog: %s, %d 'th slot", dialogID, i)
			}
			slotsGoal[slot.Group+"."+slot.Name] = true
			groupID := strconv.Itoa(slot.ID) + "." + slot.Group
			if _, ok := slotGroups[groupID]; !ok {
				slotGroups[groupID] = &SlotGroup{
					ID:           slot.ID,
					GroupName:    slot.Group,
					SlotNames:    make(map[string]*SlotIdentifier),
					SrcDialogues: map[string]bool{dialogID: true},
				}
			} else { // record src dialog id
				slotGroups[groupID].SrcDialogues[dialogID] = true
			}
			slotIdentifier := SlotIdentifier{
				FullName: slot.Group + "." + slot.Name,
			}
			slotIdentifier.IsMulti = slot.IsMulti(dialogID, i)

			if saved, ok := slotGroups[groupID].SlotNames[slot.Name]; ok {
				log.Printf("duplicate slot? %s in group: %s", slot.Name, slot.Group)
				if saved.IsMulti != slotIdentifier.IsMulti || saved.FullName != slotIdentifier.FullName {
					log.Fatalf("same slot name, different content, dialog: %s %d 'th slot, name: %s.%s", dialogID, i, slot.Group, slot.Name)
				}
			} else {
				slotGroups[groupID].SlotNames[slot.Name] = &slotIdentifier
			}

		}
		slotsFinalGoal := make(map[string]bool)
		for i, rawSlot := range rawDialogue.FinalGoal {
			slot := crosswoz.ParseSlot(rawSlot, dialogID, i)
			if !slot.Filled {
				log.Printf("!slot in final goal should be filled, dialog: %s, %d 'th slot, slot: %s.%s", dialogID, i, slot.Group, slot.Name)
			}
			slotsFinalGoal[slot.Group+"."+slot.Name] = true
		}
		if len(slotsFinalGoal) != len(slotsGoal) {
			log.Fatalf("goal and final goal slot size are different, dialog: %s, %d %d", dialogID, len(slotsFinalGoal), len(slotGroups))
		}
	}

	return groupSlots(slotGroups, inputDir, inputFile)
}

func main() {
	inputDir := "data/crosswoz/"
	inputFileName := "demo2303"
	//inputFileName := "test"
	//inputFileName := "train"

	outputDir := "agents"
	b, err := ioutil.ReadFile(path.Join(inputDir, inputFileName+".json"))
	if err != nil {
		log.Fatal("Failed to read file, err:", err)
	}
	var rawDialogues map[string]*crosswoz.RawDialogue
	if err := json.Unmarshal(b, &rawDialogues); err != nil {
		log.Fatal("Failed to unmarshal rawDialogues, err:", err)
	}
	listDomainCombinations(rawDialogues, inputFileName)
	var dialogues []*crosswoz.Dialogue
	for dialogID, rawDialogue := range rawDialogues {
		dialogue := crosswoz.TransformDialogue(dialogID, rawDialogue)
		dialogues = append(dialogues, dialogue)
	}
	sort.Slice(dialogues, func(i, j int) bool {
		return dialogues[i].DialogueID < dialogues[j].DialogueID
	})
	mergedSlotGroups := AnalyseGoals(dialogues, inputDir, inputFileName)

	GenerateIntents(mergedSlotGroups, inputFileName, outputDir)

	fileName := path.Join(inputDir, "processed_"+inputFileName+".json")
	b, err = json.MarshalIndent(dialogues, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal dialogues, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write dialogues, err:", err)
	} else {
		log.Println("wrote dialogues to file:", fileName)
	}

	//ExtractExpressions(dialogues, outputDir, inputFileName)

	dialog.AnalyseUserTurns(dialogues, inputFileName, outputDir)
	dialog.AnalyseUserTurnsREQUEST(dialogues, inputFileName, outputDir)
}
