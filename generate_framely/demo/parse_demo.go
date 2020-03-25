package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path"
	"sort"
	"strconv"
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
	TaskDescription []string `json:"task description"`
	Type            string   `json:"type"`
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
					savedGroup.SrcDialogues = append(savedGroup.SrcDialogues, group.SrcDialogues...)
					needAppend = false
				}
			}
			if needAppend {
				groups[rawGroup.GroupName] = append(groups[rawGroup.GroupName], group)
				log.Printf("same name, new appended group: %v", *group)
			}

		}
	}
	fileName := path.Join(dir, "processed_slots_grouped_"+inputFileName+".json")
	b, err := json.MarshalIndent(groups, "", "  ")
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
	var dialogues []*Dialogue
	slotGroups := make(map[string]*SlotGroup)
	for dialogID, rawDialogue := range rawDialogues {
		dialogue := &Dialogue{
			UserTurns:       int(rawDialogue.SysUsr[0]),
			SysTurns:        int(rawDialogue.SysUsr[1]),
			DialogueID:      dialogID,
			TaskDescription: rawDialogue.TaskDescription,
			Type:            rawDialogue.Type,
		}
		slotsGoal := make(map[string]bool)
		for i, rawSlot := range rawDialogue.Goal {
			slot := ParseSlot(rawSlot, dialogID, i)
			if slot.Filled {
				log.Printf("!slot in goal should not be filled, dialog: %s, %d 'th slot", dialogID, i)
			}
			dialogue.Slots = append(dialogue.Slots, slot)
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
			if slot.Values.Multi != nil {
				slotIdentifier.IsMulti = true
			} else if slot.Values.Single != nil {
				slotIdentifier.IsMulti = false
			} else {
				log.Fatalf("both single and multi are nil, dialog: %s %d 'th slot, name: %s.%s", dialogID, i, slot.Group, slot.Name)
			}
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
			dialogue.FilledSlots = append(dialogue.FilledSlots, slot)
			slotsFinalGoal[slot.Group+"."+slot.Name] = true
		}
		if len(slotsFinalGoal) != len(slotsGoal) {
			log.Fatalf("goal and final goal slot size are different, dialog: %s, %d %d", dialogID, len(slotsFinalGoal), len(slotGroups))
		}
		for k := range slotsFinalGoal {
			if _, ok := slotsGoal[k]; !ok {
				log.Fatalf("final goal slot not found in goal: %s, dialog: %s", k, dialogID)
			}
		}
		dialogues = append(dialogues, dialogue)
	}
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

	groupSlots(slotGroups, dir, inputFileName)
}
