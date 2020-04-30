package generate

import (
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"github.com/naturali/CrossWOZ/generate_framely/crosswoz"
	"log"
	"strings"
)

// go through dialogues to generate expressions
// based on the generated agent
func GenerateExpressions(allIntentIDs map[string]bool, allSlotIDs map[string]map[string]bool, dialogues []*crosswoz.Dialogue) (expressions []*p.FramelyExpression) {
	for _, dialog := range dialogues {
		for _, turn := range dialog.Turns {
			if turn.Speaker != "usr" {
				continue
			}
			exps := ExtractExpressions(turn)
			expressions = append(expressions, exps...)
		}
	}
	return expressions
}

type InformedSlotValues struct {
	Intent     string
	SlotValues map[string]string
}

type DialogueActsDetail struct {
	RequestedIntents   []string                       // intents of all the requested slots
	RequestedSlots     map[string]map[string]bool     // requested slots
	GeneralIntents     []string                       // related general intents including thank/greet/goodbye
	InformedSlotValues map[string]*InformedSlotValues // filled slot values for each intent
}

func ParseDialogueActDetail(turn *crosswoz.Message) *DialogueActsDetail {
	requestedIntents := make(map[string]bool)
	informedSlotsGroupedByIntent := make(map[string]*InformedSlotValues)
	requestedSlotsGroupedByIntent := make(map[string]map[string]bool)
	detail := &DialogueActsDetail{}
	for _, act := range turn.DialogActs {
		if act.Act == "General" {
			detail.GeneralIntents = append(detail.GeneralIntents, act.Intent)
		} else if act.Act == "Select" { // TODO select 有什么用？
		} else if act.Act == "Request" {
			requestedIntents[act.Intent] = true
			if _, ok := requestedSlotsGroupedByIntent[act.Intent]; !ok {
				requestedSlotsGroupedByIntent[act.Intent] = map[string]bool{
					act.Slot: true,
				}
			} else {
				requestedSlotsGroupedByIntent[act.Intent][act.Slot] = true
			}

		} else if act.Act == "Inform" {
			if _, ok := informedSlotsGroupedByIntent[act.Intent]; !ok {
				informedSlotsGroupedByIntent[act.Intent] = &InformedSlotValues{
					Intent:     act.Intent,
					SlotValues: map[string]string{act.Slot: act.Value},
				}
			} else {
				informedSlotsGroupedByIntent[act.Intent].SlotValues[act.Slot] = act.Value
			}
		}
	}
	detail.RequestedIntents = crosswoz.MapKeysSorted(requestedIntents)
	detail.InformedSlotValues = informedSlotsGroupedByIntent
	detail.RequestedSlots = requestedSlotsGroupedByIntent
	return detail
}

func ExtractExpressions(turn *crosswoz.Message) (expressions []*p.FramelyExpression) {
	detail := ParseDialogueActDetail(turn)
	// triggering intent
	triggeringIntent := ""
	if len(detail.RequestedIntents) > 1 {
		log.Fatal("Triggers multiple intents?", turn.Utterance, detail.RequestedIntents)
	}

	// 尝试将utterance中的空格去掉
	turn.Utterance = strings.Replace(turn.Utterance, " ", "", -1)
	// utterance里的中文括号替换成英文括号
	turn.Utterance = strings.Replace(turn.Utterance, "（", "(", -1)
	turn.Utterance = strings.Replace(turn.Utterance, "）", ")", -1)

	if len(detail.RequestedIntents) == 1 {
		triggeringIntent = detail.RequestedIntents[0]
		//log.Println("triggering intent:", triggeringIntent)
		exp := &p.FramelyExpression{
			OwnerId:   triggeringIntent,
			Utterance: turn.Utterance,
		}
		// 是不是真的第一次触发intent
		if _, ok := detail.RequestedSlots[triggeringIntent]["名称"]; !ok && triggeringIntent != "地铁" && triggeringIntent != "出租" {
			exp.Context = &p.ExpressionContext{
				FrameId: triggeringIntent,
			}
		} else {
			//exp.Context = &p.ExpressionContext{FrameId: "triggering"}
		}
		if slots, ok := detail.InformedSlotValues[triggeringIntent]; ok {
			exp.Annotations = ExtractSlotAnnotations(turn.Utterance, slots.SlotValues, triggeringIntent)
		} else {
			//log.Println("requesting with no informed slots~~~~~~~~~~~~~~~", turn.Utterance)
		}
		expressions = append(expressions, exp)
	}

	// non triggering intents
	for intent, slots := range detail.InformedSlotValues {
		booleanExpressions := BooleanExpressions(turn.Utterance, slots)
		if len(booleanExpressions) > 0 {
			expressions = append(expressions, booleanExpressions...)
		}
		if intent == triggeringIntent { // 上面已经处理过
			continue
		}
		//log.Println("informed intent:", intent)
		exp := &p.FramelyExpression{
			OwnerId: intent,
			Context: &p.ExpressionContext{
				FrameId: intent,
			},
			Utterance:   turn.Utterance,
			Annotations: ExtractSlotAnnotations(turn.Utterance, slots.SlotValues, intent),
		}
		expressions = append(expressions, exp)
	}
	return expressions
}

func BooleanExpressions(utterance string, values *InformedSlotValues) (expressions []*p.FramelyExpression) {
	for slotName, slotValue := range values.SlotValues {
		if !IsBoolean(slotName, slotValue) {
			continue
		}
		exp := &p.FramelyExpression{
			OwnerId:   "System.Boolean",
			Utterance: utterance,
		}
		if slotValue == "是" {
			exp.Label = "YES"
		} else {
			exp.Label = "NO"
		}
		exp.Context = &p.ExpressionContext{
			FrameId:     values.Intent,
			AttributeId: values.Intent + "." + slotName,
		}
		expressions = append(expressions, exp)
	}
	return expressions
}

func IsBoolean(slotName, slotValue string) bool {
	return slotValue == "是" && slotValue != "否"
}

// find slot annotations
func ExtractSlotAnnotations(utterance string, slots map[string]string, intent string) (annotations []*p.SlotAnnotation) {
	originUtterance := utterance

	for slotName, slotValue := range slots {
		if IsBoolean(slotName, slotValue) {
			continue
		}
		slotValue = strings.Replace(slotValue, " ", "", -1)
		slotValue = strings.Replace(slotValue, "（", "(", -1)
		slotValue = strings.Replace(slotValue, "）", ")", -1)
		fr, to := findSpan(utterance, slotName, slotValue)
		cnt := 0
		for fr != -1 {
			cnt++
			if cnt > 1 {
				log.Println("~~~~~~~~~~~~~~~ more than one slot values", originUtterance, slotValue)
			}
			annotations = append(annotations, &p.SlotAnnotation{
				Fr:    int32(fr),
				To:    int32(to),
				Label: intent + "." + slotName,
			})
			utterance = utterance[0:fr] + strings.Repeat("#", len(slotValue)) + utterance[(fr+len(slotValue)):]
			fr, to = findSpan(utterance, slotName, slotValue)
		}
		if cnt == 0 {
			log.Println("!!!!can not find slot value: ", utterance, " slotValue:", slotValue, " slot:", slotName)
		}
	}
	return annotations
}
