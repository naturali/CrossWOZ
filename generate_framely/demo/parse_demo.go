package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"
)

type SelectedResult []interface{}
type Message struct {
	Content   string     `json:"content"`
	DialogAct [][]string `json:"dialog_act"`
	Role      string     `json:"role"`
	SysState  map[string]interface{}
}

type RawDialogue struct {
	SysUsr          []int64         `json:"sys-usr"`
	Goal            [][]interface{} `json:"goal"`
	Messages        []*Message
	FinalGoal       [][]interface{} `json:"final_goal"`
	TaskDescription []string        `json:"task description"`
	Type            string          `json:"type"`
}

type SlotValues struct {
	Single *string   `json:"single,omitempty"`
	Multi  *[]string `json:"multi,omitempty"`
}

func (sv *SlotValues) ParseSlotValues(v interface{}) bool {
	if s, ok := v.(string); ok {
		sv.Single = &s
		return true
	}
	if a, ok := v.([]interface{}); ok {
		var values []string
		for _, value := range a {
			if s, ok := value.(string); ok {
				values = append(values, s)
			} else {
				log.Println("value not good: ", value)
				return false
			}
		}
		sv.Multi = &values
		return true
	}
	return false
}

type Slot struct {
	ID     int // TODO
	Group  string
	Name   string
	Values *SlotValues
	Filled bool // TODO what's this?
}

func (slot *Slot) IsMulti(dialogID string, i int) bool {
	if slot.Values.Multi != nil {
		return true
	} else if slot.Values.Single != nil {
		return false
	} else {
		log.Fatalf("both single and multi are nil, dialog: %s %d 'th slot, name: %s.%s", dialogID, i, slot.Group, slot.Name)
		return false
	}
}
func ParseSlot(rawSlot []interface{}, dialogID string, idx int) *Slot {
	slot := &Slot{
		Values: new(SlotValues),
	}
	// id
	if v, ok := rawSlot[0].(float64); !ok {
		log.Fatalf("parse id failed, dialog: %s, %d 'th slot: %v", dialogID, idx, rawSlot[0])
	} else {
		slot.ID = int(v)
	}
	// group
	if v, ok := rawSlot[1].(string); !ok {
		log.Fatalf("parse group failed, dialog: %s, %d 'th slot", dialogID, idx)
	} else {
		slot.Group = v
	}
	// name
	if v, ok := rawSlot[2].(string); !ok {
		log.Fatalf("parse name failed, dialog: %s, %d 'th slot", dialogID, idx)
	} else {
		slot.Name = v
	}
	// value
	if !slot.Values.ParseSlotValues(rawSlot[3]) {
		log.Fatalf("parse slot values failed, dialog: %s, %d 'th slot", dialogID, idx)
	}

	// filled
	if v, ok := rawSlot[4].(bool); !ok {
		log.Fatalf("parse filled failed, dialog: %s, %d 'th slot", dialogID, idx)
	} else {
		slot.Filled = v
	}
	return slot
}

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

type Dialogue struct {
	UserTurns   int     // TODO check
	SysTurns    int     // TODO check
	Slots       []*Slot // slots to fill? TODO check
	FilledSlots []*Slot // filled slots after dialogues? TODO check
	DialogueID  string  `json:"dialogue_id"`
	// TODO 一个description相当于一个intent？
	TaskDescription []string        `json:"task description"`
	Type            string          `json:"type"`
	Goal            [][]interface{} `json:"-"`
	Messages        []*Message      `json:"-"`
	FinalGoal       [][]interface{} `json:"-"`
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

func groupSlots(slotGroups map[string]*SlotGroup, dir string, inputFileName string) {
	var groups = make(map[string][]*MergedSlotGroup)
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
	fileName := path.Join(dir, "processed_slots_grouped_"+inputFileName+".json")
	b, err := json.MarshalIndent(groupList, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal slots, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write slots, err:", err)
	} else {
		log.Println("wrote slots to", fileName)
	}

	fileName = path.Join(dir, "processed_slots_"+inputFileName+".json")
	b, err = json.MarshalIndent(slotGroups, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal slots, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write slots, err:", err)
	} else {
		log.Println("wrote slots to", fileName)
	}
}

func listDomainCombinations(rawDialogues map[string]*RawDialogue, inputFile string) {
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
			slot := ParseSlot(rawSlot, dialogID, i)
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

func ExtractExpressions() {

}

func TransformDialogue(rawDialogue *RawDialogue) *Dialogue {
	return nil
}

func AnalyseGoals(dialogs []*Dialogue, dir string, inputFile string) {
	slotGroups := make(map[string]*SlotGroup)
	for _, rawDialogue := range dialogs {
		dialogID := rawDialogue.DialogueID
		slotsGoal := make(map[string]bool)
		for i, rawSlot := range rawDialogue.Goal {
			slot := ParseSlot(rawSlot, dialogID, i)
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
			slot := ParseSlot(rawSlot, dialogID, i)
			if !slot.Filled {
				log.Printf("!slot in final goal should be filled, dialog: %s, %d 'th slot, slot: %s.%s", dialogID, i, slot.Group, slot.Name)
			}
			slotsFinalGoal[slot.Group+"."+slot.Name] = true
		}
		if len(slotsFinalGoal) != len(slotsGoal) {
			log.Fatalf("goal and final goal slot size are different, dialog: %s, %d %d", dialogID, len(slotsFinalGoal), len(slotGroups))
		}
	}

	groupSlots(slotGroups, dir, inputFile)
}

func main() {
	dir := "data/crosswoz/"
	inputFileName := "test"
	b, err := ioutil.ReadFile(path.Join(dir, inputFileName+".json"))
	if err != nil {
		log.Fatal("Failed to read file, err:", err)
	}
	var rawDialogues map[string]*RawDialogue
	if err := json.Unmarshal(b, &rawDialogues); err != nil {
		log.Fatal("Failed to unmarshal rawDialogues, err:", err)
	}
	listDomainCombinations(rawDialogues, inputFileName)
	var dialogues []*Dialogue
	for dialogID, rawDialogue := range rawDialogues {
		dialogue := &Dialogue{
			UserTurns:       int(rawDialogue.SysUsr[0]),
			SysTurns:        int(rawDialogue.SysUsr[1]),
			DialogueID:      dialogID,
			TaskDescription: rawDialogue.TaskDescription,
			Type:            rawDialogue.Type,
			Goal:            rawDialogue.Goal,
			FinalGoal:       rawDialogue.FinalGoal,
		}
		for i, rawSlot := range rawDialogue.Goal {
			slot := ParseSlot(rawSlot, dialogID, i)
			if slot.Filled {
				log.Printf("!slot in goal should not be filled, dialog: %s, %d 'th slot", dialogID, i)
			}
			dialogue.Slots = append(dialogue.Slots, slot)
		}
		dialogues = append(dialogues, dialogue)
	}
	sort.Slice(dialogues, func(i, j int) bool {
		return dialogues[i].DialogueID < dialogues[j].DialogueID
	})
	AnalyseGoals(dialogues, dir, inputFileName)
	fileName := path.Join(dir, "processed_"+inputFileName+".json")
	b, err = json.MarshalIndent(dialogues, "", "  ")
	if err != nil {
		log.Fatal("Failed to marshal dialogues, err:", err)
	}
	if err := ioutil.WriteFile(fileName, b, 0666); err != nil {
		log.Fatal("Failed to write dialogues, err:", err)
	} else {
		log.Println("wrote dialogues to file:", fileName)
	}

}
