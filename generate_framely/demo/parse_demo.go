package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path"
	"strconv"
)

type SelectedResult []interface{}
type Message struct {
	Content   string     `json:"content"`
	DialogAct [][]string `json:"dialog_act"`
	Role      string     `json:"role"`
	SysState map[string]interface{}
}

type RawDialogue struct {
	SysUsr          []int64  `json:"sys-usr"`
	Goal      [][]interface{} `json:"goal"`
	Messages []*Message
	FinalGoal [][]interface{} `json:"final_goal"`
	TaskDescription []string `json:"task description"`
	Type string `json:"type"`
}

type SlotValues struct {
	Single string `json:"single,omitempty"`
	Multi []string `json:"multi,omitempty"`
}
func (sv *SlotValues) ParseSlotValues(v interface{}) bool {
	if s, ok := v.(string); ok {
		sv.Single = s
	} else if a, ok := v.([]interface{}); ok {
		for _, value := range a {
			if s, ok := value.(string); ok {
				sv.Multi = append(sv.Multi, s)
			} else {
				log.Println("value not good: ", value)
				return false
			}
		}
	} else {
		return false
	}
	return true
}

type Slot struct {
	ID int // TODO
	Group string
	Name string
	Values *SlotValues
	Filled bool // TODO what's this?
}

func ParseSlot(rawSlot []interface{}, dialogID string, idx int) *Slot {
	slot := &Slot{
		Values: new(SlotValues),
	}
	// id
	if v, ok := rawSlot[0].(float64); !ok {
		log.Fatalf("parse id failed, dialog: %s, %d 'th slot", dialogID, idx, rawSlot[0])
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

type SlotGroup struct {
	ID int
	GroupName string
	SlotNames map[string]bool
}

type Dialogue struct {
	UserTurns   int     // TODO check
	SysTurns    int     // TODO check
	Slots       []*Slot // slots to fill? TODO check
	FilledSlots []*Slot // filled slots after dialogues? TODO check
	DialogueID  string  `json:"dialogue_id"`
	// TODO 一个description相当于一个intent？
	TaskDescription []string `json:"task description"`
	Type string `json:"type"`
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
			slotsGoal[slot.Group + "." + slot.Name] = true
			groupID := strconv.Itoa(slot.ID) + "." + slot.Group
			if _, ok := slotGroups[groupID]; !ok {
				slotGroups[groupID] = &SlotGroup{
					ID:        slot.ID,
					GroupName: groupID,
					SlotNames: make(map[string]bool),
				}
			}
			if slot.ID != slotGroups[groupID].ID {
				log.Fatalf("same group: %s %s different ids: %d %d, dialog: %s %d 'th slot",
					slot.Group, slot.Name, slot.ID, slotGroups[slot.Group].ID, dialogID, i)
			}
			if _, ok := slotGroups[groupID].SlotNames[slot.Name]; ok {
				log.Printf("duplicate slot? %s in group: %s", slot.Name, slot.Group)
			}
			slotGroups[groupID].SlotNames[slot.Name] = true
		}
		slotsFinalGoal := make(map[string]bool)
		for i, rawSlot := range rawDialogue.FinalGoal {
			slot := ParseSlot(rawSlot, dialogID, i)
			if !slot.Filled {
				log.Printf("!slot in final goal should be filled, dialog: %s, %d 'th slot, slot: %s.%s", dialogID, i, slot.Group, slot.Name)
			}
			dialogue.FilledSlots = append(dialogue.FilledSlots, slot)
			slotsFinalGoal[slot.Group + "." + slot.Name] = true
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
