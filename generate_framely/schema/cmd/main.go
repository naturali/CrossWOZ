package main

import "github.com/krystollia/CrossWOZ/generate_framely/schema"

func main() {
	schema.GenerateAgent("data/crosswoz/database", "agents")
}
