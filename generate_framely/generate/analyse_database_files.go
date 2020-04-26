package generate

import (
	"encoding/json"
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Analyse crosswoz/database files to generate agent, entities

type RawEntry []interface{}

type Domain struct {
	Domain string
	// entity name -> entity detail
	Entities map[string]*Entity
}

type Entity struct {
	Name           string
	IsMulti        bool
	PossibleValues map[string]bool
	Domains        []string
}

func entityID(domainName string, entityName string) string {
	if entityName == "名称" {
		return domainName + "名称"
	}
	if strings.HasPrefix(entityName, "周边") {
		return strings.TrimPrefix(entityName, "周边") + "名称"
	}
	if strings.HasPrefix(entityName, "酒店设施-") {
		return "System.Boolean"
	}
	return entityName
}

var agentName = "crossDomain"

func (domain *Domain) IntentMeta() *p.IntentMeta {
	if domain.Domain == "出租车" {
		intentName := "呼叫出租车"
		intentID := "出租"
		return &p.IntentMeta{
			MetaId: intentID,
			Name:   intentName,
			Type:   "intent",
			Slots: []*p.FramelySlot{
				{
					AttributeId:   intentID + ".出发地",
					Name:          "出发地",
					TypeId:        "System.地名",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"从哪里出发？"},
				},
				{
					AttributeId:   intentID + ".目的地",
					Name:          "目的地",
					TypeId:        "System.地名",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"去哪里？"},
				},
				{
					AttributeId:   intentID + ".车型",
					Name:          "车型",
					TypeId:        "System.String",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"什么车型？"},
				},
				{
					AttributeId:   intentID + ".车牌",
					Name:          "车牌",
					TypeId:        "System.String",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"车牌是多少？"},
				},
			},
		}
	}

	if domain.Domain == "地铁" {
		intentName := "查询地铁"
		intentID := "地铁"
		return &p.IntentMeta{
			MetaId: intentID,
			Name:   intentName,
			Type:   "intent",
			Slots: []*p.FramelySlot{
				{
					AttributeId:   intentID + "." + "出发地",
					Name:          "出发地",
					TypeId:        "System.String", // TODO? 什么类型，XX名称？父类？
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"从哪里出发？"},
				},
				{
					AttributeId:  intentID + "." + "出发地附近地铁站",
					Name:         "出发地附近地铁站",
					TypeId:       "System.String", // TODO? 什么类型，地铁站？
					AllowAskSlot: false,
				},
				{
					AttributeId:   intentID + "." + "目的地",
					Name:          "目的地",
					TypeId:        "System.String", // TODO? 什么类型，XX名称？父类？
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"到哪里？"},
				},
				{
					AttributeId:  intentID + "." + "目的地附近地铁站",
					Name:         "目的地附近地铁站",
					TypeId:       "System.String", // TODO? 什么类型，XX名称？父类？
					AllowAskSlot: false,
				},
			},
		}
	}

	intentName := "找" + domain.Domain
	intentID := domain.Domain
	intent := &p.IntentMeta{
		MetaId: intentID,
		Name:   intentName,
		Type:   "intent",
	}
	// slots
	for _, ent := range domain.Entities {
		slot := &p.FramelySlot{
			AttributeId:     intent.MetaId + "." + ent.Name,
			Name:            ent.Name,
			TypeId:          entityID(domain.Domain, ent.Name),
			AllowAskSlot:    true,
			AskSlotPrompt:   []string{domain.Domain + "的" + ent.Name + "是什么？"},
			AllowMultiValue: ent.IsMulti,
		}
		if slot.TypeId == "System.Boolean" {
			slot.AskSlotPrompt = []string{"有" + ent.Name + "吗？"}
		}
		if slot.Name == "地铁" {
			slot.AskSlotPrompt = []string{
				"这个" + domain.Domain + "附近的地铁站是哪个？",
			}
		}
		if slot.Name == "门票" {
			slot.AskSlotPrompt = []string{
				"这个" + domain.Domain + "的门票多少钱？",
			}
		}
		if slot.Name == "评分" {
			slot.AskSlotPrompt = []string{
				"这个" + domain.Domain + "的评分是多少？",
			}
		}
		intent.Slots = append(intent.Slots, slot)
		if ent.IsMulti {
			slot.AskSlotPrompt = []string{domain.Domain + "的" + ent.Name + "有哪些？"}
			slot.MultiValuePrompts = []string{"还有哪些" + ent.Name}
		}
	}
	sort.Slice(intent.Slots, func(i, j int) bool {
		return intent.Slots[i].Name < intent.Slots[j].Name
	})
	return intent
}

func outputEntityExamples(ent *Entity, outputDir string) {
	os.MkdirAll(path.Join(outputDir, agentName), 0755)
	fileName := path.Join(outputDir, agentName, ent.Name) + ".entity"
	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal("Failed to open entity file to write, err:", err)
	}
	for value := range ent.PossibleValues {
		if _, err := f.WriteString(value + "\n"); err != nil {
			log.Fatal("Failed to write entity, err:", err)
		}
	}
	log.Println("Wrote entity values to", fileName)
}

func BasicTypeMetas(entities map[string]*Entity, outputDir string) []*p.BasicTypeMeta {
	var typeMetas []*p.BasicTypeMeta
	// add System.Boolean
	entities["System.Boolean"] = &Entity{
		Name:           "System.Boolean",
		PossibleValues: map[string]bool{"True": true, "False": true},
		IsMulti:        false,
	}
	for _, ent := range entities {
		if strings.HasPrefix(ent.Name, "酒店设施-") {
			// 酒店设施-XXX 都是 boolean 类型
			continue
		}
		if strings.HasPrefix(ent.Name, "周边") {
			// 周边XX 只是slot 不是entity
			continue
		}
		typeMeta := &p.BasicTypeMeta{
			TypeId:        ent.Name,
			TypeName:      ent.Name,
			IsDynamic:     false,
			IsCategorical: false, // TODO 可能需要手动修正
		}
		if ent.Name == "System.地名" {
			// 抽象的"地名"，可以是景点名称、酒店名称、餐馆名称
			typeMeta.Sons = []string{
				"景点名称",
				"酒店名称",
				"餐馆名称",
			}
		}
		// categorical?
		if len(ent.PossibleValues) > 0 && len(ent.PossibleValues) < 10 {
			typeMeta.IsCategorical = true
		}
		typeMetas = append(typeMetas, typeMeta)
		outputEntityExamples(ent, outputDir)
	}
	sort.Slice(typeMetas, func(i, j int) bool {
		if strings.HasPrefix(typeMetas[i].TypeName, "System.") && !strings.HasPrefix(typeMetas[j].TypeName, "System.") {
			return true
		} else if !strings.HasPrefix(typeMetas[i].TypeName, "System.") && strings.HasPrefix(typeMetas[j].TypeName, "System.") {
			return false
		} else {
			return typeMetas[i].TypeName < typeMetas[j].TypeName
		}
	})
	return typeMetas
}

func GenerateAgent(inputDir string, outputDir string) *p.Agent {
	fileInfos, err := ioutil.ReadDir(inputDir)
	if err != nil {
		log.Fatal(err)
	}
	var domainFiles []string
	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		if !strings.HasSuffix(fileName, "_db.json") {
			continue
		}
		domainFiles = append(domainFiles, path.Join(inputDir, fileName))
	}

	agent := &p.Agent{
		Agent: &p.DHLAgentMeta{
			AgentId: "crossWOZ",
			Name:    agentName,
		},
	}
	var domainNames []string

	allEntities := make(map[string]*Entity)
	for _, domainFile := range domainFiles {
		domain := ReadADomain(domainFile)
		domainNames = append(domainNames, domain.Domain)
		agent.Intents = append(agent.Intents, domain.IntentMeta())
		for _, ent := range domain.Entities {
			entityName := ent.Name
			if ent.Name == "名称" {
				// 不同domain的名称不能合并（如 景点名称 酒店名称）
				entityName = domain.Domain + "名称"
				ent.Name = entityName
			}
			log.Println("entity: ----", ent.Name)
			if _, ok := allEntities[entityName]; !ok {
				allEntities[entityName] = ent
				allEntities[entityName].Domains = append(allEntities[entityName].Domains, domain.Domain)
			} else {
				// 合并 possible values
				for value := range ent.PossibleValues {
					allEntities[entityName].PossibleValues[value] = true
				}
				// 检查 is multi 是否一致
				if ent.IsMulti != allEntities[entityName].IsMulti {
					log.Fatal("is multi not the same")
				}
			}
		}
	}

	sort.Slice(agent.Intents, func(i, j int) bool {
		return agent.Intents[i].MetaId < agent.Intents[j].MetaId
	})
	agent.Entities = BasicTypeMetas(allEntities, outputDir)
	agent.Agent.Description = "由 CrossWOZ 生成的 agent，涉及如下领域：" + strings.Join(domainNames, ",")
	return agent
}

func ReadADomain(fileName string) *Domain {
	if strings.HasSuffix(fileName, "taxi_db.json") {
		// 出租车 特殊处理一下
		return &Domain{
			Domain: "出租车",
			Entities: map[string]*Entity{
				"System.String": {
					Name:    "System.String",
					IsMulti: false,
					PossibleValues: map[string]bool{
						"#CX": true,
						"#CP": true,
					},
				},
			},
		}
	}
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatal("Failed to read ", fileName, err)
	}
	var rawEntries []*RawEntry
	if err := json.Unmarshal(b, &rawEntries); err != nil {
		log.Fatal("Failed to unmarshal ", fileName, err)
	}
	domain := &Domain{
		Entities: make(map[string]*Entity),
	}

	log.Println("----- parsing ", fileName)
	for _, rawEntry := range rawEntries {
		ParseRawEntry(rawEntry, domain)
	}
	return domain
}

func ParseRawEntry(rawEntry *RawEntry, domain *Domain) {
	if len(*rawEntry) != 2 {
		log.Fatal("invalid data", rawEntry)
	}
	name := (*rawEntry)[0].(string)
	kvs := (*rawEntry)[1].(map[string]interface{})
	if name != kvs["名称"] {
		log.Fatal("name not agree", name, kvs)
	}
	if domain.Domain == "" {
		domain.Domain = kvs["领域"].(string)
	}
	if domain.Domain != kvs["领域"].(string) {
		log.Fatal("Domain not agree", domain.Domain, kvs)
	}
	for k, v := range kvs {
		if k == "领域" {
			continue
		}
		if k == "酒店设施" { // 酒店设施 特殊处理
			log.Println("~~~ 酒店设施", v, kvs["名称"])
			for _, subEntity := range v.([]interface{}) {
				entityName := "酒店设施-" + subEntity.(string)
				if _, ok := domain.Entities[entityName]; !ok {
					domain.Entities[entityName] = &Entity{
						Name:    entityName,
						IsMulti: false,
					}
				}
			}
			continue
		}
		if k == "推荐菜" { // 推荐菜,多值处理
			entityName := "推荐菜"
			if _, ok := domain.Entities[entityName]; !ok {
				domain.Entities[entityName] = &Entity{
					Name:           entityName,
					IsMulti:        true,
					PossibleValues: map[string]bool{},
				}
			}
			for _, dish := range v.([]interface{}) {
				domain.Entities[entityName].PossibleValues[dish.(string)] = true
			}
			continue
		}
		if domain.Domain == "地铁" { // 地铁 domain 特殊处理
			if k == "名称" {
				k = "System.地名"
			} else if k == "地铁" {
				k = "地铁站名"
			} else {
				log.Fatal(domain.Domain, k)
			}
		}
		if k == "地铁" {
			// 其他domain中的地铁，只是上下文关联关系，并不属于其他domain的slot，因为对话中没有对比如 "景点.地铁" 的Inform或Request或Select操作
			continue
		}
		ent := domain.Entities[k]
		if _, ok := domain.Entities[k]; !ok {
			ent = &Entity{
				Name:           k,
				PossibleValues: map[string]bool{},
			}
			domain.Entities[k] = ent
		} else {
			isMulti := false
			if _, ok := v.([]interface{}); ok {
				isMulti = true
			}
			if isMulti != ent.IsMulti {
				log.Fatal("IsMulti not agree", kvs)
			}
		}
		switch v.(type) {
		case float64:
			ent.PossibleValues[strconv.FormatFloat(v.(float64), 'f', 0, 64)] = true
		case string:
			ent.PossibleValues[v.(string)] = true
		case []interface{}:
			ent.IsMulti = true
			if !strings.HasPrefix(k, "周边") {
				log.Fatal("what entity has multi-value?", k)
			}
		case nil:
			//log.Println("!!!!!!!!!!!!!!!!!!!!!null", k, v, name)
		default:
			log.Fatal("Unknown value type", k, v, name)
		}
	}
}
