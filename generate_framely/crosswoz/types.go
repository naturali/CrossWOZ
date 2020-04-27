package crosswoz

import (
	"log"
	"sort"
	"strconv"
)

type SelectedResult []interface{}
type DialogAct struct {
	Act    string
	Intent string
	Slot   string
	Value  string
}

type Message struct {
	Utterance  string
	DialogActs []*DialogAct
	UserState  []*Slot `json:"-"`
	Speaker    string  // usr or sys
}

type RelatedSlots struct {
	InformedSlots  map[string]string
	RequestedSlots []string
}

func (turn *Message) RelatedSlots() *RelatedSlots {
	var relatedSlots = &RelatedSlots{
		InformedSlots: make(map[string]string),
	}
	for _, act := range turn.DialogActs {
		if act.Intent == "greet" ||
			act.Intent == "thank" {
			continue
		}
		slotName := act.Intent + "." + act.Slot
		if act.Act == "Inform" {
			relatedSlots.InformedSlots[slotName] = act.Value
		} else if act.Act == "Request" {
			relatedSlots.RequestedSlots = append(relatedSlots.RequestedSlots, slotName)
		}
	}
	return relatedSlots
}

func (turn *Message) RelatedIntents() []string {
	var intents []string
	var intentMap = make(map[string]bool)
	for _, act := range turn.DialogActs {
		if act.Intent == "greet" ||
			act.Intent == "thank" {
			continue
		}
		intentMap[act.Intent] = true
	}
	for k := range intentMap {
		intents = append(intents, k)
	}
	sort.Strings(intents)
	return intents
}

func (turn *Message) GetDialogActs() []string {
	actMap := make(map[string]bool)
	for _, act := range turn.DialogActs {
		actMap[act.Act] = true
	}
	var acts []string
	for act := range actMap {
		acts = append(acts, act)
	}
	sort.Strings(acts)
	return acts
}

type RawMessage struct {
	Content      string          `json:"content"`
	RawDialogAct [][]string      `json:"dialog_act"`
	Role         string          `json:"role"`
	UserState    [][]interface{} `json:"user_state"`
	SysState     map[string]interface{}
	SysStateInit map[string]interface{}
}

type RawDialogue struct {
	SysUsr          []int64         `json:"sys-usr"`
	Goal            [][]interface{} `json:"goal"`
	Messages        []*RawMessage
	FinalGoal       [][]interface{} `json:"final_goal"`
	TaskDescription []string        `json:"task description"`
	Type            string          `json:"type"`
}

type Dialogue struct {
	UserVirtualID int     // TODO check
	SysVirtualID  int     // TODO check
	Slots         []*Slot // slots to fill? TODO check
	FilledSlots   []*Slot // filled slots after dialogues? TODO check
	DialogueID    string  `json:"dialogue_id"`
	// TODO 一个description相当于一个intent？
	TaskDescription []string        `json:"task description"`
	Type            string          `json:"type"`
	Goal            [][]interface{} `json:"-"`
	Turns           []*Message      `json:"turns"`
	FinalGoal       [][]interface{} `json:"-"`
}

func TransformDialogue(dialogID string, rawDialogue *RawDialogue) *Dialogue {
	dialogue := &Dialogue{
		UserVirtualID:   int(rawDialogue.SysUsr[0]),
		SysVirtualID:    int(rawDialogue.SysUsr[1]),
		DialogueID:      dialogID,
		TaskDescription: rawDialogue.TaskDescription,
		Type:            rawDialogue.Type,
		Goal:            rawDialogue.Goal,
		FinalGoal:       rawDialogue.FinalGoal,
		Turns:           make([]*Message, len(rawDialogue.Messages)),
	}
	for i, rawSlot := range rawDialogue.Goal {
		slot := ParseSlot(rawSlot, dialogID, i)
		if slot.Filled {
			log.Printf("!slot in goal should not be filled, dialog: %s, %d 'th slot", dialogID, i)
		}
		dialogue.Slots = append(dialogue.Slots, slot)
	}
	// Turns
	for msgIdx, msg := range rawDialogue.Messages {
		turn := &Message{
			Speaker:   msg.Role,
			Utterance: msg.Content,
		}
		dialogue.Turns[msgIdx] = turn
		// user state
		for slotIdx, rawSlot := range msg.UserState {
			slot := ParseSlot(rawSlot, dialogID+".Messages."+strconv.Itoa(msgIdx), slotIdx)
			turn.UserState = append(turn.UserState, slot)
		}
		// dialog act
		for _, act := range msg.RawDialogAct {
			dialogAct := &DialogAct{
				Act:    act[0],
				Intent: act[1],
				Slot:   act[2],
				Value:  act[3],
			}
			turn.DialogActs = append(turn.DialogActs, dialogAct)
		}

	}
	return dialogue
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

func MapKeysSorted(m map[string]bool) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
