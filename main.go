package main

import (
	"rsccli/cmd"
)

func main() {
	cmd.Execute()
	//rdb := redis.NewClient(&redis.Options{
	//	Addr: fmt.Sprintf("%s:%s", "localhost", "6379"),
	//	DB:   0,
	//})
	//p := tea.NewProgram(mainMenu.InitialModel(rdb))
	//if res, err := p.Run(); err != nil {
	//	fmt.Printf("Alas, there's been an error: %v", err)
	//	os.Exit(1)
	//} else {
	//	fmt.Println(res.(mainMenu.Model).Choice)
	//}

}
