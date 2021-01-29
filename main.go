package main

import (
	_ "PriceFeedTool/routers"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/toolbox"
)

func main() {
	startTask()
	beego.Run()
}
//A flag that prevents repetitive execution of timed tasks
var isTask = true
//Perform timed tasks
func startTask() {
	fmt.Println("")
	//The execution time of a timed startTast, executed every 10 seconds
	tk := toolbox.NewTask("PriceFeedTask", "0/10 * * * * *", func() error {
		if isTask {
			isTask = false
			SynAeBlock()
			isTask = true
		} else {
		}
		return nil
	})
	toolbox.AddTask("PriceFeedTask", tk)
	toolbox.StartTask()
	fmt.Println("#########  PRICE TOOL START SUCCESS #########")

}
