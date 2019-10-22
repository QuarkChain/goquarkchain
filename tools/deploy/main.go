package deploy

var (
	configPath = "./deployConfig.json"
)

func getToolManager() *ToolManager {
	config := LoadConfig(configPath)
	toolManger := NewToolManager(config)
	return toolManger
}

func main() {
	//t:=&ToolManager{
	//
	//}
	//fmt.Println(t)
}
