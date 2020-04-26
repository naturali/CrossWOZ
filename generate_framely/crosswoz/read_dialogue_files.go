package crosswoz

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"sort"
)

func ListRawDialogues(inputFileFullPath string) map[string]*RawDialogue {
	b, err := ioutil.ReadFile(inputFileFullPath)
	if err != nil {
		log.Fatal("Failed to read file, err:", err)
	}
	var rawDialogues map[string]*RawDialogue
	if err := json.Unmarshal(b, &rawDialogues); err != nil {
		log.Fatal("Failed to unmarshal rawDialogues, err:", err)
	}
	return rawDialogues

}

func ReadDialogues(inputFileFullPath string) []*Dialogue {
	rawDialogues := ListRawDialogues(inputFileFullPath)
	var dialogues []*Dialogue
	for dialogID, rawDialogue := range rawDialogues {
		dialogue := TransformDialogue(dialogID, rawDialogue)
		dialogues = append(dialogues, dialogue)
	}
	sort.Slice(dialogues, func(i, j int) bool {
		return dialogues[i].DialogueID < dialogues[j].DialogueID
	})
	return dialogues
}
