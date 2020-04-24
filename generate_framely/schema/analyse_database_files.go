package schema

import (
	"encoding/json"
	"github.com/framely/sgdnlu/generate_framely/framely"
	"github.com/framely/sgdnlu/generate_framely/framely/p"
	"io/ioutil"
	"log"
	"os"
	"path"
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
		return domainName + ".名称"
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
	if domain.Domain == "出租" {
		intentName := "呼叫出租车"
		return &p.IntentMeta{
			MetaId: agentName + "." + intentName,
			Name:   intentName,
			Type:   "intent",
			Slots: []*p.FramelySlot{
				{
					AttributeId:   intentName + ".出发地",
					Name:          "出发地",
					TypeId:        "System.地名",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"从哪里出发？"},
				},
				{
					AttributeId:   intentName + ".目的地",
					Name:          "目的地",
					TypeId:        "System.地名",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"去哪里？"},
				},
				{
					AttributeId:   intentName + ".车型",
					Name:          "车型",
					TypeId:        "System.String",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"什么车型？"},
				},
				{
					AttributeId:   intentName + ".车牌",
					Name:          "车牌",
					TypeId:        "System.String",
					AllowAskSlot:  true,
					AskSlotPrompt: []string{"车牌是多少？"},
				},
			},
		}
	}
	intentName := "找" + domain.Domain
	intent := &p.IntentMeta{
		MetaId: agentName + "." + intentName,
		Name:   intentName,
		Type:   "intent",
	}
	// slots
	for _, ent := range domain.Entities {
		intent.Slots = append(intent.Slots, &p.FramelySlot{
			AttributeId:       intent.MetaId + "." + ent.Name,
			Name:              ent.Name,
			TypeId:            entityID(domain.Domain, ent.Name),
			AllowAskSlot:      true,
			AskSlotPrompt:     []string{domain.Domain + "的" + ent.Name + "是什么？"},
			AllowMultiValue:   ent.IsMulti,
			MultiValuePrompts: []string{"还有什么" + ent.Name},
		})
	}
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
		// categorical?
		if len(ent.PossibleValues) > 0 && len(ent.PossibleValues) < 10 {
			typeMeta.IsCategorical = true
		}
		typeMetas = append(typeMetas, typeMeta)
		outputEntityExamples(ent, outputDir)
	}
	return typeMetas
}

func GenerateAgent(inputDir string, outputDir string) {
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

	agent.Entities = BasicTypeMetas(allEntities, outputDir)
	agent.Agent.Description = "由 CrossWOZ 生成的 agent，涉及如下领域：" + strings.Join(domainNames, ",")
	framely.OutputAgent(agent, outputDir)

}

func ReadADomain(fileName string) *Domain {
	if strings.HasSuffix(fileName, "taxi_db.json") {
		// 出租车 特殊处理一下
		return &Domain{
			Domain: "出租车",
			Entities: map[string]*Entity{
				"System.地名": {
					Name:    "System.地名",
					IsMulti: false,
				},
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
		if k == "推荐菜" { // 推荐菜
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
